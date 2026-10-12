# Guideline URL reference tables

For each finding, look up the matching guideline URL from the tables below. **Every finding MUST have a URL.** If no exact match exists, use the general URL for the tool or convention source.

## Effective Go conventions (22 URLs)

| Convention | URL |
|-----------|-----|
| Formatting | https://go.dev/doc/effective_go#formatting |
| Package names | https://go.dev/doc/effective_go#package-names |
| Getters | https://go.dev/doc/effective_go#getters |
| Interface names | https://go.dev/doc/effective_go#interface-names |
| MixedCaps | https://go.dev/doc/effective_go#mixedcaps |
| If | https://go.dev/doc/effective_go#if |
| For | https://go.dev/doc/effective_go#for |
| Switch | https://go.dev/doc/effective_go#switch |
| Multiple return values | https://go.dev/doc/effective_go#multiple-return-values |
| Named result parameters | https://go.dev/doc/effective_go#named-result-parameters |
| Defer | https://go.dev/doc/effective_go#defer |
| Allocation with new | https://go.dev/doc/effective_go#allocation-with-new |
| Slices | https://go.dev/doc/effective_go#slices |
| Maps | https://go.dev/doc/effective_go#maps |
| Pointer vs value receivers | https://go.dev/doc/effective_go#pointers-vs-values |
| Interfaces | https://go.dev/doc/effective_go#interfaces |
| Embedding | https://go.dev/doc/effective_go#embedding |
| Share by communicating | https://go.dev/doc/effective_go#share-by-communicating |
| Channels | https://go.dev/doc/effective_go#channels |
| Errors | https://go.dev/doc/effective_go#errors |
| Panic | https://go.dev/doc/effective_go#panic |
| Recover | https://go.dev/doc/effective_go#recover |

## Go Code Review Comments (31 URLs)

| Rule | URL |
|------|-----|
| Gofmt | https://go.dev/wiki/CodeReviewComments#gofmt |
| Comment Sentences | https://go.dev/wiki/CodeReviewComments#comment-sentences |
| Contexts | https://go.dev/wiki/CodeReviewComments#contexts |
| Copying | https://go.dev/wiki/CodeReviewComments#copying |
| Crypto Rand | https://go.dev/wiki/CodeReviewComments#crypto-rand |
| Declaring Empty Slices | https://go.dev/wiki/CodeReviewComments#declaring-empty-slices |
| Doc Comments | https://go.dev/wiki/CodeReviewComments#doc-comments |
| Don't Panic | https://go.dev/wiki/CodeReviewComments#dont-panic |
| Error Strings | https://go.dev/wiki/CodeReviewComments#error-strings |
| Examples | https://go.dev/wiki/CodeReviewComments#examples |
| Goroutine Lifetimes | https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes |
| Handle Errors | https://go.dev/wiki/CodeReviewComments#handle-errors |
| Imports | https://go.dev/wiki/CodeReviewComments#imports |
| Import Blank | https://go.dev/wiki/CodeReviewComments#import-blank |
| Import Dot | https://go.dev/wiki/CodeReviewComments#import-dot |
| In-Band Errors | https://go.dev/wiki/CodeReviewComments#in-band-errors |
| Indent Error Flow | https://go.dev/wiki/CodeReviewComments#indent-error-flow |
| Initialisms | https://go.dev/wiki/CodeReviewComments#initialisms |
| Interfaces | https://go.dev/wiki/CodeReviewComments#interfaces |
| Line Length | https://go.dev/wiki/CodeReviewComments#line-length |
| Mixed Caps | https://go.dev/wiki/CodeReviewComments#mixed-caps |
| Named Result Parameters | https://go.dev/wiki/CodeReviewComments#named-result-parameters |
| Naked Returns | https://go.dev/wiki/CodeReviewComments#naked-returns |
| Package Comments | https://go.dev/wiki/CodeReviewComments#package-comments |
| Package Names | https://go.dev/wiki/CodeReviewComments#package-names |
| Pass Values | https://go.dev/wiki/CodeReviewComments#pass-values |
| Receiver Names | https://go.dev/wiki/CodeReviewComments#receiver-names |
| Receiver Type | https://go.dev/wiki/CodeReviewComments#receiver-type |
| Synchronous Functions | https://go.dev/wiki/CodeReviewComments#synchronous-functions |
| Useful Test Failures | https://go.dev/wiki/CodeReviewComments#useful-test-failures |
| Variable Names | https://go.dev/wiki/CodeReviewComments#variable-names |

## Tool findings (8 URLs)

| Tool / Linter | URL |
|--------------|-----|
| go vet (general) | https://pkg.go.dev/cmd/vet |
| golangci-lint (general) | https://golangci-lint.run/docs/linters/ |
| golangci-lint: govet | https://pkg.go.dev/cmd/vet |
| golangci-lint: staticcheck | https://staticcheck.dev/docs/checks/ |
| golangci-lint: errcheck | https://github.com/kisielk/errcheck |
| golangci-lint: ineffassign | https://github.com/gordonklaus/ineffassign |
| golangci-lint: unused | https://staticcheck.dev/docs/checks/#U |
| go test failures | Link to the specific test file in the repo |

## K8s conventions (12 URLs)

| Convention | URL |
|-----------|-----|
| Reconciler idempotency | https://book.kubebuilder.io/reference/good-practices#why-should-reconciliations-be-idempotent |
| Level-based reconciliation | https://book.kubebuilder.io/reference/good-practices#what-is-reconciliation-in-operators |
| Single CRD per controller | https://book.kubebuilder.io/reference/good-practices#why-you-should-adhere-to-the-kubernetes-api-conventions-and-standards |
| Return conventions | https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/reconcile |
| Context propagation | https://go.dev/wiki/CodeReviewComments#contexts |
| Owner references | https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/ |
| Spec/Status separation | https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |
| Status conditions | https://book.kubebuilder.io/reference/good-practices#why-you-should-adopt-status-conditions |
| No phase fields | https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties |
| API conventions (general) | https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md |
| Finalizer pattern | https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/ |
| Status subresource | https://book.kubebuilder.io/reference/generating-crd#status |
