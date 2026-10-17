export type Health = { state: 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | 'disabled'; observed_at: string; stale_after_seconds: number; source: string }
export type Metrics = { cpu_percent?: number; ram_percent?: number; vcpu?: number; ram_gb?: number }
export type Node = { id: string; name: string; role: string; platform: string; health: Health; freshness: 'fresh' | 'stale' | 'unknown'; capabilities: string[]; metrics: Metrics; simulated: boolean }
export type Service = { id: string; name: string; health: Health; freshness: 'fresh' | 'stale' | 'unknown'; simulated: boolean }
export type Activity = { id: string; title: string; detail: string; state: string; occurred_at: string; simulated: boolean }
export type System = { name: string; site_name: string; server_time: string; demo_mode: boolean; auth_required?: boolean }
export type List<T> = { items: T[] }
