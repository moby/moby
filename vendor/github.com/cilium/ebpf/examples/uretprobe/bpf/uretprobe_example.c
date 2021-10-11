#include "common.h"
#include "bpf_helpers.h"

char __license[] SEC("license") = "Dual MIT/GPL";

struct event_t {
	u32 pid;
	char str[80];
};

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

SEC("uretprobe/bash_readline")
int uretprobe_bash_readline(struct pt_regs *ctx) {
	struct event_t event;

	event.pid = bpf_get_current_pid_tgid();
	bpf_probe_read(&event.str, sizeof(event.str), (void *)PT_REGS_RC(ctx));

	bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));

	return 0;
}
