package analyze

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/inecas/kube-health/pkg/eval"
	"github.com/inecas/kube-health/pkg/print"
	"github.com/inecas/kube-health/pkg/status"
)

var (
	gkPod              = schema.GroupKind{Group: "", Kind: "Pod"}
	progressingTimeout = 3 * time.Minute
)

type PodAnalyzer struct {
	e *eval.Evaluator
}

func (_ PodAnalyzer) Supports(obj *status.Object) bool {
	return obj.GroupVersionKind().GroupKind() == gkPod
}

func (a PodAnalyzer) Analyze(ctx context.Context, obj *status.Object) status.ObjectStatus {
	conditions, err := AnalyzeObjectConditions(obj, DefaultConditionAnalyzers)
	if err != nil {
		return status.UnknownStatusWithError(obj, err)
	}

	var pod corev1.Pod
	err = FromUnstructured(obj.Unstructured.Object, &pod)
	if err != nil {
		return status.UnknownStatusWithError(obj, err)
	}
	conditions = append(conditions, podSyntheticConditions(&pod)...)

	// We treat the containers as sub-objects of the pod, even though technically
	// they are just fields of the pod object. This makes it easier to report
	// details of each container separately.
	containerStatuses := a.analyzePodContainers(ctx, obj, &pod)

	result := AggregateResult(obj, containerStatuses, conditions)

	// Special handling for completed pods: if all containers exited successfully (exit code 0),
	// override the result to Ok, even if Ready/ContainersReady conditions are False
	// (which is expected for completed pods)
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		allSuccess := true
		hasContainers := len(pod.Status.ContainerStatuses) > 0
		
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated == nil || cs.State.Terminated.ExitCode != 0 {
				allSuccess = false
				break
			}
		}
		
		// If all containers completed successfully, mark the pod as Ok
		if allSuccess && hasContainers {
			result.ObjStatus.Result = status.Ok
			result.ObjStatus.Status = status.Ok.String()
		}
	}

	return result
}

func podSyntheticConditions(pod *corev1.Pod) []status.ConditionStatus {
	var conditions []status.ConditionStatus

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		// Pod completed successfully - check if all containers exited with code 0
		allSuccess := true
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			// All containers completed successfully, mark as Ok
			conditions = append(conditions, SyntheticConditionOk("Succeeded", ""))
		} else {
			// Some containers failed despite pod being in Succeeded phase
			conditions = append(conditions, SyntheticConditionError("Succeeded", "ContainersFailed", ""))
		}
	case corev1.PodFailed:
		// Check if this is a Job/CronJob pod that actually succeeded (all containers exit code 0)
		allSuccess := true
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated == nil || cs.State.Terminated.ExitCode != 0 {
				allSuccess = false
				break
			}
		}
		if allSuccess && len(pod.Status.ContainerStatuses) > 0 {
			// All containers exited with code 0, this is actually a success
			conditions = append(conditions, SyntheticConditionOk("Completed", ""))
		} else {
			// Actual failure
			conditions = append(conditions, SyntheticConditionError("Failed", "Failed", ""))
		}
	}

	return conditions
}

func (a PodAnalyzer) analyzePodContainers(ctx context.Context, obj *status.Object, pod *corev1.Pod) []status.ObjectStatus {
	var ret []status.ObjectStatus

	for _, cs := range pod.Status.ContainerStatuses {
		containerObjStatus := a.analyzeContainer(ctx, obj, cs)
		if containerObjStatus.Object != nil {
			ret = append(ret, containerObjStatus)
		}
	}

	return ret
}

// analyzeContainer analyzes the status of a container, treating it as a separate
// sub-object of the pod.
func (a PodAnalyzer) analyzeContainer(ctx context.Context, obj *status.Object, cs corev1.ContainerStatus) status.ObjectStatus {
	containerObj := &status.Object{
		TypeMeta: metav1.TypeMeta{
			Kind: "Container",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cs.Name,
		},
	}

	conditions := []status.ConditionStatus{}
	var cond status.ConditionStatus
	if cs.State.Waiting != nil {
		var lastTransitionTime time.Time
		progressing := true
		if lastState := cs.LastTerminationState.Terminated; lastState != nil {
			lastTransitionTime = lastState.FinishedAt.Time
		}

		if !lastTransitionTime.IsZero() && time.Since(lastTransitionTime) > progressingTimeout {
			progressing = false
		}
		reason := cs.State.Waiting.Reason
		cond = SyntheticConditionError("Waiting", reason, "")
		cond.LastTransitionTime = metav1.NewTime(lastTransitionTime)
		cond.CondStatus.Progressing = progressing
	}

	if cs.State.Running != nil {
		cond = SyntheticConditionOk("Running", "")
		cond.LastTransitionTime = cs.State.Running.StartedAt
	}

	if !cs.Ready {
		cond = SyntheticConditionError("Ready", "NotReady", "")
	}

	if cs.State.Terminated != nil {
		reason := cs.State.Terminated.Reason
		// Check exit code: 0 means successful completion, non-zero means failure
		if cs.State.Terminated.ExitCode == 0 {
			cond = SyntheticConditionOk("Terminated", reason)
		} else {
			cond = SyntheticConditionError("Terminated", reason, "")
		}
	}

	if (cond == status.ConditionStatus{}) {
		return status.ObjectStatus{}
	}

	// Only fetch logs for actual errors (not for successful completions)
	if cond.Status().Result > status.Ok {
		a.expandWithLogs(ctx, obj, cs.Name, &cond)
	}

	conditions = append(conditions, cond)

	return AggregateResult(containerObj, nil, conditions)
}

// expandWithLogs loads container logs and appends them to the condition message.
func (a PodAnalyzer) expandWithLogs(ctx context.Context, obj *status.Object, container string, cond *status.ConditionStatus) {
	logs, err := a.loadContainerLogs(ctx, obj, container)
	if err != nil {
		logs = "Error loading logs: " + err.Error() + "\n"
	}

	if logs == "" {
		return
	}

	// Apply color highlighting to logs
	logs = print.HighlightLogs(logs, a.e.UseColor())

	if cond.Message != "" {
		cond.Message = "\n"
	}

	cond.Message += "Logs:\n"
	cond.Message += logs
}

func (a PodAnalyzer) loadContainerLogs(ctx context.Context, obj *status.Object, container string) (string, error) {
	logobjs, err := a.e.Load(ctx, eval.PodLogQuerySpec{
		Object:    obj,
		Container: container,
	})
	if err != nil {
		return "", err
	}

	if len(logobjs) == 0 {
		return "", nil
	}

	logs, _, _ := unstructured.NestedString(logobjs[0].Unstructured.Object, "log")
	return logs, nil
}

func init() {
	Register.Register(func(e *eval.Evaluator) eval.Analyzer {
		return PodAnalyzer{e: e}
	})
}
