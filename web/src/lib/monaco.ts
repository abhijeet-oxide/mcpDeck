// Monaco with the TypeScript language service, bundled so it works offline (UI-9).
import * as monaco from "monaco-editor";
import EditorWorker from "monaco-editor/editor/editor.worker.js?worker";
import TsWorker from "monaco-editor/languages/features/typescript/ts.worker.js?worker";

(self as any).MonacoEnvironment = {
  getWorker(_: string, label: string) {
    if (label === "typescript" || label === "javascript") return new TsWorker();
    return new EditorWorker();
  },
};

const ts = monaco.typescript;
ts.typescriptDefaults.setCompilerOptions({
  target: ts.ScriptTarget.ES2020,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.NodeJs,
  allowNonTsExtensions: true,
  strict: false,
  noEmit: true,
  lib: ["es2020"],
});
ts.typescriptDefaults.setDiagnosticsOptions({ noSemanticValidation: false, noSyntaxValidation: false });

let lib: monaco.IDisposable | null = null;
/** setTypes loads the generated mcpdeck.d.ts so `crm.` lists requests (TOOL-3). */
export function setTypes(dts: string) {
  lib?.dispose();
  lib = ts.typescriptDefaults.addExtraLib(dts, "file:///mcpdeck.d.ts");
}

export function applyTheme() {
  const dark =
    document.documentElement.dataset.theme === "dark" ||
    (!document.documentElement.dataset.theme && window.matchMedia?.("(prefers-color-scheme: dark)").matches);
  monaco.editor.setTheme(dark ? "vs-dark" : "vs");
}

export { monaco };
