import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get } from "./api";
import type {
  AgentDetail,
  AgentSummary,
  APIDetail,
  APISummary,
  AuditEvent,
  Call,
  CallDetail,
  Home,
  Info,
  SettingsView,
  ToolDetail,
  ToolkitDetail,
  ToolkitSummary,
  ToolRow,
  Usage,
  Version,
} from "./types";

export const useInfo = () => useQuery({ queryKey: ["info"], queryFn: () => get<Info>("/api/info"), staleTime: 60_000 });
export const useHome = () =>
  useQuery({ queryKey: ["home"], queryFn: () => get<Home>("/api/home"), refetchInterval: 10_000 });
export const useAPIs = () => useQuery({ queryKey: ["apis"], queryFn: () => get<APISummary[]>("/api/apis") });
export const useAPI = (id: string) =>
  useQuery({ queryKey: ["api", id], queryFn: () => get<APIDetail>(`/api/apis/${id}`), enabled: !!id });
export const useHistory = (id: string) =>
  useQuery({ queryKey: ["history", id], queryFn: () => get<Call[]>(`/api/apis/${id}/history`) });
export const useToolkits = () =>
  useQuery({ queryKey: ["toolkits"], queryFn: () => get<ToolkitSummary[]>("/api/toolkits") });
export const useToolkit = (ref: string) =>
  useQuery({
    queryKey: ["toolkit", ref],
    queryFn: () => get<ToolkitDetail>(`/api/toolkits/${ref}`),
    refetchInterval: 15_000,
    enabled: !!ref,
  });
export const useLibrary = () => useQuery({ queryKey: ["tools"], queryFn: () => get<ToolRow[]>("/api/tools") });
export const useTool = (ref: string, enabled = true) =>
  useQuery({ queryKey: ["tool", ref], queryFn: () => get<ToolDetail>(`/api/tools/${ref}`), enabled: enabled && !!ref });
export const useVersion = (id: string) =>
  useQuery({ queryKey: ["version", id], queryFn: () => get<Version>(`/api/versions/${id}`) });
export const useAgents = () =>
  useQuery({ queryKey: ["agents"], queryFn: () => get<AgentSummary[]>("/api/agents"), refetchInterval: 10_000 });
export const useAgent = (id: string, poll = false) =>
  useQuery({
    queryKey: ["agent", id],
    queryFn: () => get<AgentDetail>(`/api/agents/${id}`),
    refetchInterval: poll ? 1500 : 10_000,
    enabled: !!id,
  });
export const useCalls = (q: string) =>
  useQuery({ queryKey: ["calls", q], queryFn: () => get<Call[]>(`/api/calls?${q}`), refetchInterval: 5_000 });
export const useCall = (id: string) =>
  useQuery({ queryKey: ["call", id], queryFn: () => get<CallDetail>(`/api/calls/${id}`) });
export const useUsage = (range: string) =>
  useQuery({ queryKey: ["usage", range], queryFn: () => get<Usage>(`/api/usage?range=${range}`) });
export const useAudit = () => useQuery({ queryKey: ["audit"], queryFn: () => get<AuditEvent[]>("/api/audit") });
export const useSettings = () =>
  useQuery({ queryKey: ["settings"], queryFn: () => get<SettingsView>("/api/settings") });

/** useAction wraps a mutation that refreshes everything on success. */
export function useAction<A, R>(
  fn: (a: A) => Promise<R>,
  opts?: { onSuccess?: (r: R, a: A) => void; onError?: (e: unknown) => void },
) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: (r, a) => {
      qc.invalidateQueries();
      opts?.onSuccess?.(r, a);
    },
    onError: (e) => opts?.onError?.(e),
  });
}
