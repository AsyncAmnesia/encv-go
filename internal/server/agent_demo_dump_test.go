// internal/server/agent_demo_dump_test.go
//
// 临时调试用：跑 mock scenario，dump Run() 写到 recorder 的真实 SSE 输出。
// 用于「剧本重构」端到端 demo 验证（不只测 PASS，而是看真实输出）。
//
// 用法：go test -v -run TestAgentDemoDump -count=1 ./internal/server/
//
// 跑完即删。
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestAgentDemoDump_DefaultFriendly 跑 default_friendly 剧本，把 Run() 写到 recorder
// 的完整 SSE 流 dump 出来（每行带相对开始时间的 ms）。
func TestAgentDemoDump_DefaultFriendly(t *testing.T) {
	dumpScenario(t, "default_friendly")
}

// TestAgentDemoDump_V2_SearchRecursiveMp4 跑 v2_01（垃圾剧本代表）。
func TestAgentDemoDump_V2_SearchRecursiveMp4(t *testing.T) {
	dumpScenario(t, "search_recursive_mp4")
}

// TestAgentDemoDump_V2_BranchEncrypt 跑 v2_06 branch_encrypt_or_decrypt。
func TestAgentDemoDump_V2_BranchEncrypt(t *testing.T) {
	dumpScenario(t, "branch_encrypt_or_decrypt")
}

// TestAgentDemoDump_V2_EditMetadataWizard 跑 v2_04 edit_metadata_wizard（4 轮）。
func TestAgentDemoDump_V2_EditMetadataWizard(t *testing.T) {
	dumpScenario(t, "edit_metadata_wizard")
}

// TestAgentDemoDump_AllBuiltin 一口气跑全部 21 个 builtin 剧本，每个只打印事件计数 + 总耗时。
func TestAgentDemoDump_AllBuiltin(t *testing.T) {
	eng := NewMockEngine()
	s := newMockTestServer()
	sess := newMockSession()
	scenarios := eng.AllScenarios()

	fmt.Println("\n========== ALL 21 BUILTIN SCENARIOS (count + duration) ==========")
	for _, sc := range scenarios {
		// 重置 session
		sess.EventCache = sess.EventCache[:0]
		rec := httptest.NewRecorder()

		start := time.Now()
		ctx := context.Background()
		_ = eng.Run(ctx, s, sess, rec, rec, sc, 1.0, true) // speed=1 真实节奏
		elapsed := time.Since(start).Milliseconds()

		// 统计事件类型
		typeCount := map[string]int{}
		for _, ev := range sess.EventCache {
			typeCount[ev.Type]++
		}
		parts := []string{}
		for _, typ := range []string{"text_delta", "reasoning_delta", "tool_call", "tool_result", "stream_start", "stream_end", "mock_presets"} {
			if c := typeCount[typ]; c > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", typ, c))
			}
		}
		fmt.Printf("[%s] id=%-32s t=%4dms  events=%d  (%s)\n",
			sc.ID, sc.ID, elapsed, len(sess.EventCache), strings.Join(parts, " "))
	}
	fmt.Println("========== END ==========")
}

// dumpScenario 跑指定 ID 的剧本并 dump 真实 SSE 输出。
//
// 来源：先查 NewMockEngine() builtin（v1，13 个），再查 mockScenariosV2（v2，8 个）。
func dumpScenario(t *testing.T, scenarioID string) {
	eng := NewMockEngine()
	sc := eng.GetScenarioByID(scenarioID)
	if sc == nil {
		// v2 剧本不在 NewMockEngine builtin（v1 only），查 mockScenariosV2
		for _, v2sc := range mockScenariosV2 {
			if v2sc.ID == scenarioID {
				sc = v2sc
				break
			}
		}
	}
	if sc == nil {
		t.Fatalf("scenario %q not found in builtin (v1=%d, v2=%d)", scenarioID, len(eng.AllScenarios()), len(mockScenariosV2))
	}

	s := newMockTestServer()
	sess := newMockSession()
	rec := httptest.NewRecorder()

	start := time.Now()
	ctx := context.Background()
	if err := eng.Run(ctx, s, sess, rec, rec, sc, 1.0, true); err != nil { // speed=1 真实节奏
		t.Fatalf("Run: %v", err)
	}
	totalElapsed := time.Since(start).Milliseconds()

	fmt.Printf("\n========== SCENARIO: %s (total=%dms) ==========\n", scenarioID, totalElapsed)

	body := rec.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			compact := compactJSON(payload)
			fmt.Printf("  %s\n", compact)
		} else if line != "" {
			fmt.Printf("  [meta] %s\n", line)
		}
	}
	fmt.Println("========== END ==========")
}

// compactJSON 把 JSON 压成单行。
func compactJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}
