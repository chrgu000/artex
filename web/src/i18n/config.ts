// 支持的 locale 与默认值。界面默认使用中文，韩文作为可选语言保留。
export const LOCALES = ["zh", "ko"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "zh";

// 활성 locale 을 빌드 시점에 결정한다. 정적 내보내기(next.config 의 output: "export")와
// 호환되어야 하므로 cookies()·headers() 같은 동적 API 를 쓰지 않고 환경변수만 읽는다.
// NEXT_PUBLIC_LOCALE 为空或不在支持列表中时，回落到默认值（zh）。
export function resolveLocale(): Locale {
  const raw = process.env.NEXT_PUBLIC_LOCALE;
  return LOCALES.includes(raw as Locale) ? (raw as Locale) : DEFAULT_LOCALE;
}
