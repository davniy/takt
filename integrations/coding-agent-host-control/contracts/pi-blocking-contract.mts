import { rm, writeFile } from "node:fs/promises"
import taktHostControl from "../pi/index.js"

type Handler = (event: any, ctx: any) => Promise<any>
let activeTools = ["read", "write", "bash"]
let transformer: ((markdown: string, context: any) => string) | undefined
const handlers = new Map<string, Handler>()
const api = {
  getActiveTools: () => activeTools,
  setActiveTools: (tools: string[]) => { activeTools = tools },
  registerCommand: () => undefined,
  registerMarkdownTransformer: (value: (markdown: string, context: any) => string) => { transformer = value },
  on: (name: string, handler: Handler) => { handlers.set(name, handler) },
}
const ctx = {
  cwd: process.cwd(),
  isIdle: () => true,
  sessionManager: { getSessionId: () => "pi-contract" },
  ui: { notify: () => undefined, confirm: async () => true, setStatus: () => undefined },
}

taktHostControl(api as never)
const sessionStart = handlers.get("session_start")
if (!sessionStart) throw new Error("Pi session_start hook is missing")
await sessionStart({}, ctx)
if (!activeTools.includes("read") || activeTools.includes("write")) throw new Error(`Pi managed tool mode is wrong: ${activeTools}`)

if (!transformer) throw new Error("Pi markdown transformer is missing")
const hidden = transformer("PREMATURE_STREAM", { messageType: "assistant", isStreaming: true, availableWidth: 80 })
if (hidden.includes("PREMATURE_STREAM")) throw new Error(`Pi streamed completion was visible: ${hidden}`)

const completion = handlers.get("message_end")
if (!completion) throw new Error("Pi message_end hook is missing")
const replaced = await completion({ message: { role: "assistant", stopReason: "stop", content: [{ type: "text", text: "PREMATURE_FINAL" }] } }, ctx)
const replacementText = (replaced?.message?.content?.[0] as { text?: string } | undefined)?.text
if (replacementText !== "TAKT_COMPLETION_BLOCKED") throw new Error(`Pi premature final was not replaced: ${JSON.stringify(replaced)}`)

const tool = handlers.get("tool_call")
if (!tool) throw new Error("Pi tool_call hook is missing")
const denied = await tool({ toolName: "write" }, ctx)
if (!denied?.block || !String(denied.reason).includes("policy denied")) throw new Error(`Pi tool call was not denied: ${JSON.stringify(denied)}`)

await writeFile("daemon-down", "down\n")
try {
  const failClosed = await tool({ toolName: "read" }, ctx)
  if (!failClosed?.block || !String(failClosed.reason).includes("fail-closed")) throw new Error(`Pi daemon failure was not fail-closed: ${JSON.stringify(failClosed)}`)
  if (activeTools.length !== 0) throw new Error(`Pi tools remained enabled after daemon failure: ${activeTools}`)
} finally {
  await rm("daemon-down", { force: true })
}

console.log("Pi host blocking contract: PASS")
