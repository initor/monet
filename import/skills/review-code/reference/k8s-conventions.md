# K8s Convention Checklist (15 items)

Walk through all 15 K8s conventions below against the sampled code. Same approach as the Effective Go and Code Review Comments checklists: note violations, note good patterns, skip silently if not applicable.

## Controller Patterns (6)

1. **Reconciler idempotency** -- The Reconcile function must produce the same result regardless of how many times it is called with the same input. Check that reconcile logic does not assume it runs only once per event. Why it matters: the controller runtime may call Reconcile multiple times for the same object due to requeues, cache syncs, or leader election changes. Non-idempotent reconcilers cause drift and flapping.

2. **Level-based, not edge-based** -- The reconciler should react to the current observed state of the world, not to specific events (create, update, delete). Check that the reconcile function reads current state from the API server (via `Get` or `List`) rather than relying on event payloads. Why it matters: events can be missed, reordered, or duplicated. Level-triggered reconciliation is self-healing.

3. **Single CRD per controller** -- Each controller should manage exactly one primary resource type. Check that each reconciler struct watches one GVK. Why it matters: mixing multiple resource types in one reconciler makes error handling, requeue logic, and RBAC scoping significantly harder.

4. **Return conventions** -- Check that the Reconcile function uses the correct return patterns:
   - `ctrl.Result{}, nil` -- reconciliation succeeded, no requeue needed
   - `ctrl.Result{RequeueAfter: duration}, nil` -- requeue after a specific delay (e.g., polling external state)
   - `ctrl.Result{}, err` -- reconciliation failed, requeue with exponential backoff
   - **Deprecated pattern:** `ctrl.Result{Requeue: true}` without `RequeueAfter` -- flag this as a deprecated pattern. Prefer `RequeueAfter` with an explicit duration for predictable retry behavior.
   Why it matters: incorrect return values cause infinite tight requeue loops or missed reconciliation.

5. **Context propagation** -- The `ctx` parameter from the Reconcile method signature must be passed through to all downstream calls (API server reads, writes, logging). Check that functions called from Reconcile accept and forward `context.Context`. Why it matters: context carries cancellation signals, deadlines, and tracing metadata. Dropping context breaks graceful shutdown and observability.

6. **Owner references** -- Child resources created by the controller should have `OwnerReferences` set pointing to the parent custom resource. Check for `ctrl.SetControllerReference()` or `controllerutil.SetOwnerReference()` calls when creating dependent resources. Why it matters: owner references enable garbage collection -- when the parent is deleted, Kubernetes automatically cleans up child resources.

## API Design (6)

7. **Spec/Status separation** -- `Spec` holds the desired state (written by the user), `Status` holds the observed state (written by the controller). Check that the CRD type definitions in `*_types.go` have separate `Spec` and `Status` structs. Check that the controller does not write to `Spec` fields. Why it matters: mixing spec and status breaks the declarative model and confuses users about which fields they own.

8. **Status conditions** -- Status should use the standard conditions pattern with fields: `type` (string), `status` (metav1.ConditionStatus: True/False/Unknown), `reason` (CamelCase string), `message` (human-readable string), and `observedGeneration` (int64). Check that `*_types.go` includes a `Conditions []metav1.Condition` field. Why it matters: conditions are the standard K8s pattern for communicating multi-dimensional status. Tools like kubectl, ArgoCD, and Flux understand this format.

9. **No phase fields** -- **Deprecated pattern:** Check for `Phase` string fields in the Status struct. If found, flag as deprecated. The K8s community recommends conditions instead of phase enums because phases are single-dimensional (only one value at a time) and cannot represent partial progress or multiple concurrent states. Why it matters: phase fields have been deprecated in upstream K8s resources (e.g., Pod phase is maintained only for backward compatibility).

10. **CamelCase constants** -- K8s API constants should use CamelCase (`ClusterFirst`, `ReadWriteOnce`), not SCREAMING_SNAKE_CASE. Check enum-like constants in `*_types.go`. Why it matters: CamelCase matches the K8s API convention established by core types. SCREAMING_SNAKE_CASE looks foreign in K8s manifests.

11. **String enums over int enums** -- Use typed string constants for enumeration values rather than iota-based integer constants. Check that `type SomePhase string` is preferred over `type SomePhase int`. Why it matters: string enums are human-readable in YAML manifests, JSON output, and logs without needing a lookup table.

12. **Avoid booleans in spec** -- Spec fields should prefer string enums over booleans. Check for `bool` fields in Spec structs. Why it matters: booleans cannot be extended (what if a third option is needed later?) and their meaning is often ambiguous in YAML (`enabled: true` -- enabled what?). String enums like `mode: "active" | "passive" | "disabled"` are clearer and extensible.

## Operations (3)

13. **RBAC annotations match actual API usage** -- Check that `+kubebuilder:rbac` marker comments in the controller file match the actual Kubernetes API calls made in the reconcile function. If the controller calls `Get` on ConfigMaps, there should be a corresponding RBAC annotation granting `get` on `configmaps`. Why it matters: missing RBAC annotations cause permission errors at runtime that are hard to debug. Excess RBAC annotations grant unnecessary permissions (principle of least privilege).

14. **Finalizer pattern** -- If the controller needs to clean up external resources on deletion, check for the correct finalizer pattern:
    1. On reconcile, if the object does not have the finalizer, add it (register)
    2. On reconcile, if the object is being deleted (DeletionTimestamp is set), perform cleanup, then remove the finalizer
    3. The finalizer must be removed **last**, after cleanup succeeds
    Check for `controllerutil.AddFinalizer()` and `controllerutil.RemoveFinalizer()` usage.
    Why it matters: incorrect finalizer handling can block object deletion forever (if finalizer is never removed) or skip cleanup (if finalizer is removed before cleanup).

15. **Status subresource** -- Status should be updated via the `/status` subresource endpoint (`client.Status().Update()` or `client.Status().Patch()`), not via a regular `Update()` on the main resource. Check for `+kubebuilder:subresource:status` marker in the type definition and `r.Status().Update()` calls in the controller. Why it matters: updating the main resource to change status triggers a new reconcile (spec generation changes), creating an infinite reconcile loop. The `/status` subresource avoids this.

For each K8s convention:
- If a violation is found: note the file, approximate location, what the convention expects, and a concrete suggestion.
- If the code follows the convention well: note it as a good pattern (collect for "Good Patterns" section).
- If the convention is not applicable to the sampled code: skip it silently.
