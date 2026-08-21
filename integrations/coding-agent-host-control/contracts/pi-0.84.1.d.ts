declare module "@earendil-works/pi-coding-agent" {
  type AssistantMessage = {
    role: "assistant"
    stopReason: string
    content: unknown[]
  }

  type AgentMessage = AssistantMessage | { role: "user"; content: string | unknown[] }

  export interface MessageEndEvent {
    type: "message_end"
    message: AgentMessage
  }

  export interface ExtensionContext {
    cwd: string
    waitForIdle(): Promise<void>
    isIdle(): boolean
    sessionManager: {
      getSessionId?(): string
      getSessionFile?(): string
    }
    ui: {
      notify(message: string, level: "info" | "error"): void
      confirm(title: string, message: string): Promise<boolean>
      setStatus(key: string, value: string | undefined): void
    }
  }

  export interface ExtensionAPI {
    getActiveTools(): string[]
    setActiveTools(tools: string[]): void
    registerCommand(name: string, command: {
      description: string
      handler(args: string, ctx: ExtensionContext): Promise<void>
    }): void
    registerMarkdownTransformer(transformer: (markdown: string, context: {
      messageType: "user" | "assistant" | "assistant-thinking"
      isStreaming: boolean
      availableWidth: number
    }) => string): void
    on(name: "session_start", handler: (event: unknown, ctx: ExtensionContext) => Promise<void>): void
    on(name: "input", handler: (event: { source: string; text: string }, ctx: ExtensionContext) => Promise<{ action: "continue" | "handled" }>): void
    on(name: "tool_call", handler: (event: { toolName: string }, ctx: ExtensionContext) => Promise<void | { block?: boolean; reason?: string }>): void
    on(name: "message_end", handler: (event: MessageEndEvent, ctx: ExtensionContext) => Promise<void | { message: AgentMessage }>): void
    on(name: "user_bash", handler: (event: unknown, ctx: ExtensionContext) => Promise<void | { result: { output: string; exitCode: number; cancelled: boolean; truncated: boolean } }>): void
    on(name: "session_before_switch" | "session_before_fork", handler: (event: unknown, ctx: ExtensionContext) => Promise<void | { cancel: true }>): void
  }
}

declare module "node:child_process" {
  export function execFile(file: string, args: readonly string[], options: { maxBuffer: number }, callback: (error: Error | null, stdout: string, stderr: string) => void): void
}
declare module "node:util" {
  export function promisify<T extends Function>(fn: T): (...args: any[]) => Promise<{ stdout: string; stderr: string }>
}
declare module "node:crypto" {
  export function createHash(name: string): { update(value: string): { digest(encoding: "hex"): string } }
}
declare module "node:fs/promises" {
  export function mkdir(path: string, options: { recursive: boolean; mode: number }): Promise<void>
  export function readFile(path: string, encoding: "utf8"): Promise<string>
  export function rm(path: string, options: { force: boolean }): Promise<void>
  export function writeFile(path: string, data: string, options?: { mode?: number }): Promise<void>
}
declare module "node:path" {
  export function dirname(path: string): string
  export function join(...parts: string[]): string
}
declare const process: {
  cwd(): string
  stderr: { write(chunk: string | Uint8Array): boolean }
}
