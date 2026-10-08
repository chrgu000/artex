package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Autumn-27/artex/db"
)

const maxWorkerMessageBytes = 64 << 10

// Worker 개입(사람이 작성한 메시지를 일시정지된 Worker 의도에 주입) API 의 사용자 노출
// 에러 응답 문구다. 한국어 UI 에서 토스트로 그대로 노출되므로 한국어로 둔다. Worker 는
// 엔진 역할 이름이라 로마자를 유지하고(용어집), request_id 는 요청 필드명이라 원문 보존.
const (
	errIntentRequestTooLarge   = "请求正文过大"
	errIntentMessageEmpty      = "消息不能为空"
	errIntentMessageTooLong    = "消息不能超过 4000 个字符"
	errIntentBadRequestID      = "request_id 必须是 1-128 位字母、数字、-、_、. 或 :"
	errIntentTaskDeleting      = "任务正在删除，无法向 Worker 发送消息"
	errIntentTaskPaused        = "任务已暂停，请先恢复任务再向 Worker 发送消息"
	errIntentTaskQueued        = "排队中的任务无法向 Worker 发送消息"
	errIntentTaskTerminal      = "终态任务无法向 Worker 发送消息"
	errIntentTaskSettling      = "任务正在收尾，无法向 Worker 发送消息"
	errIntentInheritedReadonly = "继承意图为只读，不能发送 Worker 消息"
	errIntentNotPaused         = "仅已暂停的 Worker 可以发送消息，请先暂停"
)

func validWorkerMessageRequestID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ':' {
			continue
		}
		return false
	}
	return true
}

// sendWorkerMessage continues a paused Worker intent with a human-authored message.
// The message is injected as the next turn's input through the same
// resume-from-transcript path the worker uses on a normal resume (ExecuteWithMessage),
// so this reuses the existing pause/resume machinery rather than a bespoke protocol.
// The run happens in a dedicated goroutine outside the worker pool (runDetachedIntent),
// so the message is picked up immediately even when every pool slot is busy — the same
// way the main-agent chat handler starts its run directly. In-memory only: a process
// restart re-runs the intent from its transcript without the message, which is
// acceptable for this rare interrupt-then-continue action.
func (s *Server) sendWorkerMessage(w http.ResponseWriter, r *http.Request) {
	t, ok := s.m.Task(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "task not found")
		return
	}
	iid, err := strconv.ParseInt(r.PathValue("iid"), 10, 64)
	if err != nil || iid <= 0 {
		writeErr(w, http.StatusBadRequest, "bad intent id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWorkerMessageBytes)
	var req struct {
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeErr(w, http.StatusRequestEntityTooLarge, errIntentRequestTooLarge)
			return
		}
		writeErr(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	message := strings.TrimSpace(req.Message)
	requestID := strings.TrimSpace(req.RequestID)
	if message == "" {
		writeErr(w, http.StatusBadRequest, errIntentMessageEmpty)
		return
	}
	if len([]rune(message)) > 4000 {
		writeErr(w, http.StatusBadRequest, errIntentMessageTooLong)
		return
	}
	if !validWorkerMessageRequestID(requestID) {
		writeErr(w, http.StatusBadRequest, errIntentBadRequestID)
		return
	}

	// Reject non-runnable task lifecycles up front so the caller gets a clear reason
	// instead of a silent no-op. The intent itself must be paused: the UI flow is
	// interrupt (pause) first, then send.
	if s.engine.IsDeleting(t.ID) {
		writeErr(w, http.StatusConflict, errIntentTaskDeleting)
		return
	}
	lifecycle := t.lifecycleSnapshot()
	switch {
	case lifecycle.Paused || s.engine.IsPaused(t.ID):
		writeErr(w, http.StatusConflict, errIntentTaskPaused)
		return
	case lifecycle.Queued:
		writeErr(w, http.StatusConflict, errIntentTaskQueued)
		return
	case isTerminalStatus(lifecycle.Status):
		writeErr(w, http.StatusConflict, errIntentTaskTerminal)
		return
	case s.engine.isSettling(t.ID):
		writeErr(w, http.StatusConflict, errIntentTaskSettling)
		return
	}

	node, err := t.Store.GetNode(iid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node == nil {
		if inherited, sourceErr := t.Store.GetNodeWithSources(iid); sourceErr == nil && inherited != nil && inherited.Inherited {
			writeErr(w, http.StatusConflict, errIntentInheritedReadonly)
			return
		}
		writeErr(w, http.StatusNotFound, "intent not found")
		return
	}
	if node.Kind != db.KindIntent {
		writeErr(w, http.StatusConflict, "node is not an intent")
		return
	}
	if node.State != "paused" {
		writeErr(w, http.StatusConflict, errIntentNotPaused)
		return
	}
	agentMessage, ok := s.prepareChatMentionMessage(w, message)
	if !ok {
		return
	}

	// runDetachedIntent transitions paused->running, emits the user turn and starts a
	// dedicated run. Root the run at s.ctx so a disconnected browser cannot strand it
	// while task pause/delete/shutdown still stop it.
	if err := s.engine.runDetachedIntent(s.ctx, t, iid, requestID, message, agentMessage); err != nil {
		switch {
		case errors.Is(err, db.ErrIntentStateConflict):
			writeErr(w, http.StatusConflict, err.Error())
		default:
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         iid,
		"state":      "running",
		"accepted":   true,
		"request_id": requestID,
	})
}
