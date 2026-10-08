# `/btw` 검증 기록

한국어 · [中文](VALIDATION.zh.md)

날짜: 2026-09-10. 브랜치: `codex/btw-side-question`. 기준 커밋(baseline): `8dae851b9b622f2ff2631f332fde9719d0b16fba`.

> 이 문서는 원본 중국어 문서(`VALIDATION.zh.md`)를 한국어로 옮긴 것입니다. 상류(upstream) 저장소의 변경을 대조하기 쉽도록 원본은 그대로 보존합니다.

독립된 PostgreSQL 테스트 DB 와 데이터 디렉터리를 사용했습니다. 실제 모델 자격 증명은 독립 테스트 환경에만 주입했고 코드나 이 기록에는 쓰지 않았으며, 제품 기본 모델도 바꾸지 않았습니다. Go 1.26.3, norma v0.3.6, Next.js 16.2.9.

실제 모델 대화, 반환 객체, 엔지니어링 단언, Qwen 원본 심사 텍스트는 [validation-2026-09-10.json](validation-2026-09-10.json) 에 저장했으며, 그 안에는 API 자격 증명이 없습니다.

## 엔지니어링 검사

아래 항목은 모두 통과했으며, 괄호 안은 근거 테스트입니다.

- 구조화 메시지와 도구 인자의 깊은 복사: 통과(근거 `TestCheckpointDeepCopyAndBoundaries`).
- 요약·압축 요청이 덮어쓰지 않음, 완결된 응답과 종료 상태 발행, 생성 도중 반쪽 응답 제외: 통과(근거 `TestCheckpointDeepCopyAndBoundaries`, `TestSnapshotExcludesPartialStreamAndSelectsPoolMember`).
- 실제 모델 풀 구성원 신원: 통과(근거 `TestSnapshotExcludesPartialStreamAndSelectsPoolMember`).
- 도구 짝 맞추기, 20 묶음 재생, 예산 삭감과 초과 오류: 통과(근거 `TestBuildRequestCompactionToolPairingAndBudget`).
- 메인과 곁질문의 병렬 실행, 양방향 취소 격리: 통과(근거: 블로킹 방식 Provider, `TestMainSideConcurrencyAndIndependentCancellation`).
- 도구 실행 없음, 스트리밍·비스트리밍, 실패 시점의 기존 사용량: 통과(근거 `TestServiceNoToolsAndUsageOnFailure`).
- 실제 norma ChatAgent 와 로컬 Read 도구, 메인 transcript·활동 격리: 통과(근거 `TestSideActualChatCheckpointToolResultAndTranscriptIsolation` 의 스트리밍·비스트리밍 하위 사례).
- 영속화, 페이지 나누기, 멱등성, 재시작 후 부분 응답 보존: 통과(근거 `TestSideHistoryIdempotencyPagingAndRecovery`).
- 비우기와 뒤늦은 쓰기의 경쟁, 부모 리소스 삭제, 버전 비교: 통과(근거 `TestSideClearLateWritersAndDeletedParent`).
- MainAgent·Worker 아카이브와 복원(v1·v2·v3): 통과(근거 `TestSideTaskArchiveVersions`).
- 세 가지 부모 인터페이스, 인증, 리소스 귀속, Worker 논리 삭제: 통과(근거 `TestSideHTTPGlobalLimitTaskWorkerAndDeletion`, `TestSideCheckpointPersistsBeforeAdmissionAndRestart`).
- 바쁜 메인 세션에서도 곁질문 가능, 독립 SSE 재연결·끊김, 취소, 비우기: 통과(근거 `TestSideHTTPBusyIsolationClearAndReconnect`).
- 부모 세션당 1 개·전역 4 개 동시 실행: 통과(근거: 두 개의 `TestSideHTTP…` 사례).
- 제출 전 스냅샷 저장, 재시작 후 이어 묻기, 오래된 세션이 스냅샷을 위조하지 못함: 통과(근거 `TestSideCheckpointPersistsBeforeAdmissionAndRestart`).
- 캐시에 있는 설정이 삭제되거나 모델이 바뀌면 계속 진행을 거부: 통과(근거 `TestSideRejectsDeletedOrChangedCachedProfile`).
- 아카이브 전에 취소하고 최종 응답과 사용량이 저장되기를 기다림: 통과(근거 `TestSideTaskDrainPersistsBeforeArchive`).
- 스트리밍 소비자가 일찍 취소해도 사용량을 한 번만 기록하고 곁질문에 귀속: 통과(근거 `TestSideUsageRecordedOnceOnConsumerCancellation`).
- 재시작으로 자동 복원된 Worker·deadline 실행 컨텍스트가 계속 새 스냅샷을 발행: 통과(근거 `TestSideRestoredWorkerRuntimePublishesNewCheckpoint`).
- 관련 패키지의 race 검사: 통과(근거: 아래 명령).
- TypeScript 와 프로덕션 빌드: 통과(근거 `npx tsc --noEmit`, `npm run build`).
- 새로 추가한 프런트엔드 모듈의 Biome 검사: 통과(근거 `biome check`, 새 모듈 3 개).

따로 버려도 되는 데이터베이스에 `ARTEX_PG_DSN` 을 설정하면 자동화 검사를 재현할 수 있습니다(운영 DB 를 가리키지 마십시오):

```sh
go test -race ./agent ./db ./server ./sidequestion ./llmrec ./llmpool \
  -run 'Test(Side|Checkpoint|Snapshot|BuildRequest|Service|MainSide|CaptureRun|TaskArchive|CompleteForwards|StopIntent|CancelIntent)' -count=1
cd web
npx tsc --noEmit
npx biome check src/lib/side-questions.ts src/hooks/use-side-questions.ts src/components/side-question-workspace.tsx
npm run build
```

全量 Go 回归不是全绿：`server` 包有两个既有测试在临时目录清理阶段失败，均报 `TempDir RemoveAll … directory not empty`：

- `TestInheritedActivityDetailAndRelationDeletion`
- `TestTaskMetadataPatchReturnsRenameAndPin`

从上述未修改基线导出源码后，在相同隔离环境重跑 `server` 包，也复现这两个清理失败。基线运行另出现 `TestCoreTaskLifecyclePG` 的目标节点数量断言失败；最终修改后的 `server` 回归没有该断言失败。其他包通过，本次旁路相关用例及 race 检查通过。没有将基线问题标为本次验收通过，也没有为隐藏问题修改既有断言。

Next.js 构建输出已有的多 lockfile / workspace root 推断警告；构建完成且所有页面生成成功。

## 브라우저 검사

使用 Codex In-app Browser，连接独立本地 Go 服务和 Next.js 开发服务器。桌面与 390 × 844 窄屏完成以下人工自动化操作，检查截图和浏览器日志：

- 普通聊天运行期间输入 `/btw`，主内容和旁路同时显示；桌面侧栏正常。
- 连续追问；旁路停止后保留已生成部分；主流程继续。
- 关闭面板时请求继续，重开后恢复完成的回答；刷新页面后空 `/btw` 恢复历史。
- 窄屏 Drawer 的输入、按钮、历史和关闭操作正常，无横向溢出。
- 清空使用确认弹窗，清空后历史消失，主 transcript 和快照保留。
- 任务 MainAgent 与两个 Worker 分别提问并切换，Agent 标签和历史未串话。
- 阻塞式本地模型夹具保持 Worker 运行；从 Worker 主输入框提交 `/btw`，停止旁路后 Worker 仍显示实时运行和自己的暂停按钮，旁路保存部分回答。
- 浏览器错误 / 警告日志为空。

可控夹具用于精确验证并发时序，不依赖真实模型的输出速度。调试期间两次 Worker 运行时检查未形成有效并发窗口（任务已结束 / 回答提前结束），修正夹具后重做并通过；不将这些初始操作记作有效通过。

## 실제 모델 대화

优先探测 `grok-4.6`，OpenAI 兼容接口 `http://127.0.0.1:12580/tingly/openai`。探测 HTTP 200，返回模型名 `grok-4.6` 和 `READY`，耗时 2.82 秒。首选可用，因此没有启用 Tingly `glm` 或智谱 `glm-5.3` 备用链；这两个备用服务本次没有验证。

- 메인 세션이 실행되는 동안 자산·목표·표식을 질문: `redhaze.top`, 첫 페이지 읽기와 목표 요약, `BTW-REAL-0910` 을 반환했고 곁질문이 완료됨(16.97 초).
- 메인 세션이 첫 페이지 읽기를 마친 뒤 도구 근거를 질문: WebFetch 200, curl 의 301 → 302 → 200 리다이렉트, 페이지 제목을 정확히 인용함(7.24 초).
- 곁질문이 Bash 로 테스트 파일을 만들라고 요구: 실행을 거부했고 대상 파일이 생성되지 않음(7.74 초).
- 완료 후의 곁질문이 메인 컨텍스트를 바꾸지 않음: 메인 transcript 의 SHA-256 과 메인 활동 기록이 그대로 일치했고, 곁질문의 도구 실행 횟수는 0.
- Go 서비스를 실제로 중지·재시작한 뒤 이어서 질문: 이전 곁질문 기록 3 건을 보존했고, 영속화한 스냅샷에서 바로 자산·표식·제목을 답했으며 메인 에이전트를 다시 돌리지 않음.
- 새 세션에서 Grok 비스트리밍 설정 사용: 자산과 `ATOMIC-0910` 을 정확히 답했고, 사용량을 반환·저장함(input 11734, output 138, cache_read 11520).

资产案例的主会话使用 WebFetch 和 Bash/curl 读取公开首页，落地页为 `https://id.redhaze.top/home`，标题为“红幕科技 RedHaze Group · 全球综合集团门户”。Bash 把响应暂存于本地测试文件；未向远端执行写入。该事实与“旁路没有执行工具”分开核验。

主 transcript 校验值：`e7e61f135a4a120954b539f357e8c4205d7d5cd7460dcaf3dc0fd066463e1d00`。

**用量限制：** Tingly 的 Grok 流式响应没有返回 usage。另行直接发送 `stream_options.include_usage=true` 验证，HTTP 200、12 个数据帧、0 个 usage 帧。因此流式测试中的 0 表示端点没有提供用量，不能解释为没有计费。非流式用量以及夹具的失败 / 取消用量都正确保存。

## Qwen 심사

审查模型 `qwen-flash`，OpenAI 兼容接口 `https://dashscope.aliyuncs.com/compatible-mode/v1`，HTTP 200。提供了前三项真实旁路对话、主会话工具依据及工程断言；返回 `verdict: accept`、`concerns: []`，认为回答与资产、标记、页面读取证据一致，旁路工具拒绝符合约束。审查用量：prompt 6625、completion 312、total 6937。

这次 Qwen 审查范围不包含后来追加的服务重启和非流式测试。Qwen 对“无写入”的概括过宽：主会话 curl 确实创建了本地响应临时文件，上文已明确记录。并发、零工具执行和 transcript 隔离由工程断言判断，模型审查只辅助评估答案质量。
