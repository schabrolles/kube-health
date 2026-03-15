package analyze_test

import (
	"testing"

	"github.com/inecas/kube-health/pkg/status"
	"github.com/stretchr/testify/assert"

	"github.com/inecas/kube-health/internal/test"
)

func TestPodAnalyzer(t *testing.T) {
	var os status.ObjectStatus
	e, l, objs := test.TestEvaluator("pods.yaml")

	os = e.Eval(t.Context(), objs[0])
	assert.False(t, os.Status().Progressing)
	assert.Equal(t, os.Status().Result, status.Ok)

	l.RegisterPodLogs("default", "p2", "p2c", "Line 1\nLine 2\nLine 3\n")
	os = e.Eval(t.Context(), objs[1])
	assert.False(t, os.Status().Progressing)
	assert.Equal(t, os.Status().Result, status.Error)

	test.AssertConditions(t, `PodReadyToStartContainers   (Unknown)
Initialized   (Unknown)
Ready ContainersNotReady containers with unready status: [p2c] (Error)
ContainersReady ContainersNotReady containers with unready status: [p2c] (Unknown)
PodScheduled   (Unknown)`, os.Conditions)

	test.AssertConditions(t, `Ready NotReady Logs:
Line 1
Line 2
Line 3
 (Error)`, os.SubStatuses[0].Conditions)

 // Test Job pod with successful completion (exit code 0)
 os = e.Eval(t.Context(), objs[5])
 assert.False(t, os.Status().Progressing)
 assert.Equal(t, status.Ok, os.Status().Result, "Job pod with exit code 0 should be Ok")
 test.AssertConditions(t, `Initialized   (Unknown)
Ready PodCompleted  (Error)
ContainersReady PodCompleted  (Unknown)
PodScheduled   (Unknown)
Succeeded   (Ok)`, os.Conditions)
 // Container should be Ok with exit code 0
 assert.Equal(t, 1, len(os.SubStatuses), "Should have one container status")
 assert.Equal(t, status.Ok, os.SubStatuses[0].Status().Result, "Container with exit code 0 should be Ok")
 test.AssertConditions(t, `Terminated  Completed (Ok)`, os.SubStatuses[0].Conditions)

 // Test Job pod with failed completion (non-zero exit code)
 os = e.Eval(t.Context(), objs[6])
 assert.False(t, os.Status().Progressing)
 assert.Equal(t, status.Error, os.Status().Result, "Job pod with non-zero exit code should be Error")
 test.AssertConditions(t, `Initialized   (Unknown)
Ready PodFailed  (Error)
ContainersReady PodFailed  (Unknown)
PodScheduled   (Unknown)
Failed Failed  (Error)`, os.Conditions)
 // Container should be Error with non-zero exit code
 assert.Equal(t, 1, len(os.SubStatuses), "Should have one container status")
 assert.Equal(t, status.Error, os.SubStatuses[0].Status().Result, "Container with non-zero exit code should be Error")
 test.AssertConditions(t, `Terminated Error  (Error)`, os.SubStatuses[0].Conditions)

 // Test CronJob pod with successful completion
 os = e.Eval(t.Context(), objs[7])
 assert.False(t, os.Status().Progressing)
 assert.Equal(t, status.Ok, os.Status().Result, "CronJob pod with exit code 0 should be Ok")
 // Container should be Ok with exit code 0
 assert.Equal(t, 1, len(os.SubStatuses), "Should have one container status")
 assert.Equal(t, status.Ok, os.SubStatuses[0].Status().Result, "CronJob container with exit code 0 should be Ok")
}
