# End-to-end coverage map

The `e2e` package holds xcl's whole-library, black-box tests. They drive xcl
the way an application does, through its public packages only, against
fixtures the suite owns: configurations under `testdata/`, and block types,
an in-process plugin and an external plugin under `fixtures/`. The only
internal package the suite imports is `internal/testutil`, the shared test
helpers. `TestMain` builds the external plugin once per run into a temporary
directory and removes it when the run ends. The suite is part of xcl's root
module, so `go test ./...` from the repository root runs it.

Each example under `example/` is a Go module of its own, so the root
`go test ./...` does not reach it directly. The suite runs each example's own
tests, smoke tests included, through one named test per example (for example
`TestPluginExampleTestsPass`), and fails naming the example whose tests fail.
A new example adds its own named runner test to the suite as part of the change
that adds the example; there is deliberately no test that discovers example
directories.

The examples used to double as xcl's end-to-end tests. Their tests that
asserted what xcl did were removed once the tests below covered the same
behaviour, keeping the same assertions. The tables record, for every removed
library-behaviour test, the test that now covers it. Where one example test
checked both library behaviour and what the example or prettylog printed, the
library half moved here and the printing half stayed with the example or
prettylog.

## diff

`diff_test.go` proves that `Config.Diff` predicts `Config.Apply`. Each test
copies `testdata/plugin` or `testdata/kube` to a temporary directory, so it can
edit the files, and keeps its state in a temporary directory of its own. Every
diff is checked to leave the state file byte for byte unchanged and to report
no create, update or destroy event. The prediction tests then apply and check
the diff against what the apply's lifecycle events show it did.

The diff never under-reports and may over-report. A resource the diff reports
as an update has every computed value unknown, so each resource referencing one
is reported as an update with that value unknown and gets no provider call; the
apply may then find the value unchanged and leave that resource alone. The
prediction tests therefore check a contract rather than equality:

- an apply never changes a resource the diff did not list: every resource the
  apply creates, updates, replaces or deletes is in the diff with that same
  action
- every resource the diff lists that the apply leaves alone was listed with at
  least one change that is unknown

Each prediction test also pins the exact create, update, replace and delete
addresses its diff reports.

| Test | Behaviour |
|---|---|
| `TestDiffOfUnappliedPluginConfigurationPredictsApply` | with no state every provider resource is created |
| `TestDiffOfUnappliedPluginConfigurationReportsComputedValuesUnknown` | values computed by a provider during the apply are reported unknown |
| `TestDiffOfUnchangedPluginConfigurationReportsNoChanges` | an applied, unchanged configuration reports no changes |
| `TestDiffOfEditedRedisPortPredictsApply` | an edited attribute is an update, and what references a computed value of an updated resource is an update with that value unknown, whether or not the apply changes it |
| `TestDiffOfEditedReplicaLocationPredictsApply` | an edited attribute nothing references is an update of that resource only |
| `TestDiffOfRemovedIngressPredictsApply` | a removed block is a delete |
| `TestDiffOfFailedReplicaPredictsReplaceByApply` | a resource saved as failed is replaced |
| `TestDiffOfChangedPasswordPredictsApply` | a changed sensitive value is an update, and what references a computed value of the updated databases is an update with that value unknown although the apply leaves it alone |
| `TestDiffOfChangedPasswordReportsSensitivePasswordChange` | the change is marked sensitive and carries neither value |
| `TestDiffOfKubeConfigurationReportsNothing` | declared types only (`xcl.WithType`): no resources, every count zero |
| `TestApplyOfKubeConfigurationHandsNothingToAProvider` | declared types only (`xcl.WithType`): no provider call |
| `TestDiffOfUnappliedPluginConfigurationMarshalsNoPassword`, `TestDiffOfChangedPasswordMarshalsNoPassword` | no password in the diff's JSON |
| `TestDiffOfUnappliedPluginConfigurationFormatsNoPasswordWithV`, `TestDiffOfChangedPasswordFormatsNoPasswordWithV` | no password in the diff formatted with `%v` |
| `TestDiffOfUnappliedPluginConfigurationFormatsNoPasswordWithPlusV`, `TestDiffOfChangedPasswordFormatsNoPasswordWithPlusV` | no password in the diff formatted with `%+v` |
| `TestDiffOfUnappliedPluginConfigurationReportsNoPasswordInEvents`, `TestDiffOfChangedPasswordReportsNoPasswordInEvents` | no password in the events the diff reports |

## configonly

Removed from `example/configonly/main_test.go`.

| Removed test | Behaviour | Now covered by |
|---|---|---|
| `TestConfigOnlyExampleFindsDeclaredResources` | apply yields every declared resource | `e2e.TestApplyFindsEveryDeclaredResource` |
| `TestConfigOnlyExampleReturnsRegisteredGoTypes` | blocks held as their registered Go type | `e2e.TestApplyReturnsRegisteredGoTypes` |
| `TestConfigOnlyExampleDecodesRepeatedBlocks` | repeated blocks decode to ordered slices | `e2e.TestDecodeFillsRepeatedBlocksInOrder` |
| `TestConfigOnlyExampleDecodesNestedBlocks` | blocks nested in nested blocks decode | `e2e.TestDecodeFillsNestedBlocks` |
| `TestConfigOnlyExampleLeavesOmittedBlockNil` | omitted block is nil / empty | `e2e.TestDecodeLeavesOmittedBlockNil` |
| `TestConfigOnlyExampleReadsValuesFromConfigMap` | references into another resource's map resolve | `e2e.TestDecodeResolvesValuesReadFromAnotherResourcesMap` |
| `TestConfigOnlyExampleReadsVariables` | variables resolve, plain and interpolated | `e2e.TestDecodeResolvesVariables` |
| `TestConfigOnlyExampleLinksServiceToDeployment` | reference into repeated block by position | `e2e.TestDecodeResolvesReferenceIntoRepeatedBlockByPosition` |
| `TestConfigOnlyExampleLinksIngressToService` | reference to another resource's id and attribute | `e2e.TestDecodeResolvesReferenceToAnotherResourcesAttributes` |
| `TestConfigOnlyExampleReportsParseEventWithFile` | parse success event carries file | `e2e.TestParseEventCarriesFile` |
| `TestConfigOnlyExampleReportsCreateSuccessWithoutStart` | registered-type create reports success only | `e2e.TestCreateOfRegisteredTypeReportsSuccessWithoutStart` |
| `TestConfigOnlyExampleReportsNoLogEvents` | no log events without plugins | `e2e.TestRegisteredTypesReportNoLogEvents` |
| `TestConfigOnlyExampleReportsNoErrors` | successful lifecycle reports no errors | `e2e.TestSuccessfulLifecycleReportsNoErrors` |
| `TestConfigOnlyExampleReportsEveryEventFromCore` | every event sourced from core | `e2e.TestRegisteredTypesReportEveryEventFromCore` |
| `TestConfigOnlyExampleReportsOperationAndPhaseOnEveryEvent` | every event has operation and phase | `e2e.TestEveryEventCarriesOperationAndPhase` |
| `TestConfigOnlyExampleReportsParseErrorWithFile` | parse error event names block and file | `e2e.TestParseErrorEventNamesBlockAndFile` |
| `TestConfigOnlyExampleReportsDestroySuccessWithoutStart` | registered-type destroy reports success only | `e2e.TestDestroyOfRegisteredTypeReportsSuccessWithoutStart` |
| `TestConfigOnlyExampleReportsDestroyOperationStartAndSuccess` | destroy operation brackets resource events | `e2e.TestDestroyOperationReportsStartFirstAndSuccessLast` |
| `TestConfigOnlyExampleDestroysEverythingItApplied` | saved state empty after destroy | `e2e.TestDestroyLeavesSavedStateEmpty` |
| `TestRunWithoutReceiverWritesNothingToStdoutOrStderr (configonly)` | nil handler keeps std streams silent | `e2e.TestKubeLifecycleWithoutEventHandlerWritesNothingToStandardStreams` |
| `TestConfigOnlyExampleEntityAndStateAgree` | saved record encodes identically to applied entity | `e2e.TestSavedEntityEncodesAsAppliedEntity` |
| `TestConfigOnlyExampleDoesNotWarnAboutPlainState` | no plain-text state warning with a state mask | `e2e.TestNoPlainStateWarningWithKey` |
| `TestConfigOnlyExampleWithoutKeyWarnsAboutPlainState` | one plain-text state warning without a mask | `e2e.TestPlainStateWarningWithoutKey` |
| `TestConfigOnlyExampleReadsTheSecretFromTheEnvironment` | env() fills a Sensitive value, Reveal returns it | `e2e.TestEnvFunctionReadsSensitiveValue` |
| `TestConfigOnlyExampleReferencesTheSecretByNameAndKey` | nested secret_key_ref decodes name and key | `e2e.TestNestedBlockReferencesSecretByNameAndKey` |
| `TestConfigOnlyExampleStateHoldsNoSecret` | state with a mask holds masked envelopes, no secret | `e2e.TestStateHoldsNoSecretWithKey` |
| `TestConfigOnlyExampleEventDataHoldsNoSecret` | no event (data or otherwise) carries the secret | `e2e.TestEventsHoldNoSecret` |
| `TestConfigOnlyExampleShowsCreatedEntities` | EncodeSavedEntity of create-success data writes configuration | `e2e.TestEncodeSavedEntityWritesCreatedConfiguration` |
| `TestConfigOnlyExamplePrintsNoSecret` | xcl writes no secret to stdout/stderr; encoded event data shows SensitiveMarker, not the secret | `e2e.TestStandardStreamsHoldNoSecret`, `e2e.TestEncodeSavedEntityMasksSecret` |

## plugin

Removed from `example/plugin/main_test.go`.

| Removed test | Behaviour | Now covered by |
|---|---|---|
| `TestPluginExampleFindsDeclaredResources` | apply finds every declared resource (plugins + module) | `e2e.TestApplyFindsEveryResourceDeclaredForPlugins` |
| `TestPluginExampleFillsConnectionString` | provider fills computed connection_string (postgres, redis, module) | `e2e.TestProviderFillsComputedValue` |
| `TestPluginExamplePassesConnectionStringToReferencingBlock` | computed values cross from in-process to external plugin | `e2e.TestComputedValueCrossesPlugins` |
| `TestPluginExamplePassesComputedURLToIngress` | computed url crosses between types of one plugin | `e2e.TestComputedValueCrossesTypesOfOnePlugin` |
| `TestPluginExampleHoldsGeneratedTypes` | plugin resources held as schema-generated types | `e2e.TestPluginResourcesAreHeldAsGeneratedTypes` |
| `TestPluginExampleRetrievesPublishedValues` | Find[types.Output] returns root output value; Find[types.Output] returns module output value | `e2e.TestFindReturnsRootOutputValue`, `e2e.TestFindReturnsModuleOutputValue` |
| `TestPluginExampleReportsInProcessPluginInitAtDebug` | in-process plugin Init log at debug, load op | `e2e.TestInProcessPluginInitLogsAtDebug` |
| `TestPluginExampleReportsInProcessProviderInitAtDebug` | provider Init logs at debug naming block type | `e2e.TestInProcessProviderInitLogsAtDebug` |
| `TestPluginExampleReportsInProcessProviderCreateLogsAsCreateEvents` | in-process create logs as create events with resource and file | `e2e.TestInProcessProviderCreateLogsReportedAsCreateEvents` |
| `TestPluginExampleReportsExternalProviderCreateLogsFromThePluginBinary` | external create logs sourced from binary name | `e2e.TestExternalProviderCreateLogsSourcedFromPluginBinary` |
| `TestPluginExampleReportsExternalProviderCreateLogsBetweenStartAndSuccess` | external create log between resource start/success | `e2e.TestExternalProviderCreateLogsBetweenStartAndSuccess` |
| `TestPluginExampleReportsInProcessProviderCreateLogsBetweenStartAndSuccess` | in-process create log between resource start/success | `e2e.TestInProcessProviderCreateLogsBetweenStartAndSuccess` |
| `TestPluginExampleReportsPluginLoadingLogsAtDebug` | plugin loading logs all at debug | `e2e.TestPluginLoadingLogsAtDebug` |
| `TestPluginExampleReportsProviderCallLogsAtInfo` | provider call logs at info (12 = 6 created + destroyed) | `e2e.TestProviderCallLogsAtInfo` |
| `TestPluginExampleReportsExternalProviderDestroyLogsBetweenStartAndSuccess` | external destroy log between resource start/success | `e2e.TestExternalProviderDestroyLogsBetweenStartAndSuccess` |
| `TestPluginExampleReportsInProcessProviderDestroyLogsBetweenStartAndSuccess` | in-process destroy log between resource start/success | `e2e.TestInProcessProviderDestroyLogsBetweenStartAndSuccess` |
| `TestPluginExampleFailsWithoutExternalPlugin` | missing external plugin fails Apply with ErrPluginLoad naming path | `e2e.TestMissingExternalPluginFailsApply` |
| `TestPluginExampleReportsCreateEventPhases` | create start+success per provider resource, success only for builtins | `e2e.TestPluginCreateReportsStartAndSuccessForProviderResources` |
| `TestPluginExampleReportsPluginsLoaded` | each plugin load reported once by core | `e2e.TestPluginLoadReportedOnceByCore` |
| `TestPluginExampleReportsBlockTypesOfLoadedPlugins` | load events name plugin and block types | `e2e.TestPluginLoadNamesPluginAndBlockTypes` |
| `TestPluginExampleReportsNoErrors` | no error events, errors or error logs | `e2e.TestPluginLifecycleReportsNoErrors` |
| `TestPluginExampleReportsNoWarnings` | no warn logs with a state key | `e2e.TestPluginLifecycleWithKeyReportsNoWarnings` |
| `TestPluginExampleReportsParseEventWithFile` | parse success from core with file | `e2e.TestPluginParseEventCarriesFile` |
| `TestPluginExampleReportsDestroyEventPhases` | destroy start+success per provider resource, success only for builtins | `e2e.TestPluginDestroyReportsStartAndSuccessForProviderResources` |
| `TestPluginExampleReportsDestroyOperationStartAndSuccess` | destroy operation start first, success last, from core | `e2e.TestPluginDestroyOperationReportsStartFirstAndSuccessLast` |
| `TestPluginExampleInProcessProvidersReportDestroyForEveryResource` | in-process provider destroy log per resource | `e2e.TestInProcessProviderDestroyLogsEveryResource` |
| `TestPluginExampleExternalProvidersReportDestroyForEveryResource` | external provider destroy log per resource | `e2e.TestExternalProviderDestroyLogsEveryResource` |
| `TestPluginExampleDestroysEverythingItApplied` | raw state file is [] after destroy | `e2e.TestPluginDestroyLeavesSavedStateEmpty` |
| `TestPluginExampleConvertsExternalPluginTypes` | external plugin types encode from event data (library half) | `e2e.TestEncodeSavedEntityWritesExternalPluginTypes` |
| `TestPluginExampleEntityAndStateAgree` | saved record encodes identically to entity | `e2e.TestPluginSavedEntityEncodesAsAppliedEntity` |
| `TestPluginExampleEveryEntityConverts` | every entity encodes or is ErrNotEncodable | `e2e.TestPluginEntitiesEncodeOrAreRefusedAsNotEncodable` |
| `TestPluginExampleStateHoldsNoSecret` | state files hold no password, masked envelopes | `e2e.TestPluginStateHoldsNoSecretWithKey` |
| `TestPluginExampleEventDataHoldsNoSecret` | no event holds a password | `e2e.TestPluginEventsHoldNoSecret` |
| `TestPluginExampleWithoutKeyWarnsAboutPlainState` | plain state warning once per apply and destroy | `e2e.TestPluginPlainStateWarningWithoutKey` |
| `TestRunWithoutReceiverWritesNothingToStdoutOrStderr (plugin)` | nil handler, nothing on stdout/stderr | `e2e.TestPluginLifecycleWithoutEventHandlerWritesNothingToStandardStreams` |
| `TestPluginExampleShowsPostgresConfigurationAfterCreate` | create-success event data encodes with values, nested block, computed value (library half) | `e2e.TestEncodeSavedEntityWritesPluginConfigurationWithComputedValues` |
| `TestPluginExampleShowsEveryCreatedEntity` | every created resource encodes from event data (library half) | `e2e.TestEncodeSavedEntityWritesEveryCreatedPluginResource` |
| `TestPluginExamplePrintsNoSecret` | standard streams hold no password (library half); encoded event data masks passwords (library half) | `e2e.TestPluginStandardStreamsHoldNoSecret`, `e2e.TestEncodeSavedEntityMasksPluginPasswords` |

## Stayed with the example or prettylog

These tests check what an example prints or how it exits, not what xcl did, so
they stay with the example:

- configonly: `TestConfigOnlyExamplePrintsEveryResource`,
  `TestConfigOnlyExamplePrintsNestedBlocks`,
  `TestConfigOnlyExamplePrintsLinkedResources`,
  `TestConfigOnlyExampleFailsForMissingConfig`,
  `TestConfigOnlyExamplePrintsNoResourcesRemaining`, and the report half of
  `TestConfigOnlyExamplePrintsNoSecret`.
- plugin: `TestPluginExamplePrintsEveryResource`,
  `TestPluginExampleFailsForMissingConfig`, the "build it with `make build`"
  hint half of `TestPluginExampleFailsWithoutExternalPlugin` (the example's own
  wording), `TestPluginExamplePrintsNoResourcesRemaining`,
  `TestPluginExamplePrintsPublishedTotal` (the count behind it is also covered
  by `TestOutputsHoldsModuleOutputs`), and the report half of
  `TestPluginExamplePrintsNoSecret`.
- The rendering halves of `TestConfigOnlyExampleShowsCreatedEntities`,
  `TestPluginExampleShowsPostgresConfigurationAfterCreate`,
  `TestPluginExampleShowsEveryCreatedEntity` and
  `TestPluginExampleConvertsExternalPluginTypes` (configuration written beneath
  the line announcing the resource) are prettylog's behaviour and are covered
  by prettylog's own tests, such as
  `TestHandlerWritesConfigurationAfterCreateSuccess`.

## Source-inspection tests removed without replacement

These tests parsed an example's source to check how it was written, not what
it does. They were deleted, not carried over: a change to how an example is
written, without changing what it does, must not fail a test.

| Removed test | Where | Why no replacement |
|---|---|---|
| `TestConfigOnlyExampleImportsNoPluginCode` | `example/configonly/main_test.go` | Checked the example's imports. |
| `TestConfigOnlyExampleUsesPortableLookupForm` | `example/configonly/main_test.go` | Checked the lookup form used. That what a reader copies compiles on the minimum supported Go is now held by the CI job that builds and vets every example module on Go 1.25.0. |
| `TestConfigOnlyExampleAssemblesItsConfigurationWithDecode` | `example/configonly/main_test.go` | Checked the example called Decode. Decode itself is covered by the `TestDecode…` tests above. |
| `TestConfigOnlyExampleMakesNoPerTypeLookups` | `example/configonly/main_test.go` | Checked the absence of per-type lookups in the source. |
| `TestPluginExampleDefinesNoTypesOrConfig` | `example/plugin/main_test.go` | Checked where types and configuration were declared. |
| `TestPluginExampleUsesPortableLookupForm` | `example/plugin/main_test.go` | As the configonly test above; held by the CI minimum-Go build of each example. |
| `TestExamplesPassTheirHandlerToWithEventHandlerOnce` | `static_examples_test.go` (root) | Source scan of every example's `main.go`. |
| `TestExamplesMainSetsUpPrettyLogOnce` | `static_examples_test.go` (root) | Source scan. |
| `TestExamplesDoNotImportLogger` | `static_examples_test.go` (root) | Source scan of imports. |
| `TestExamplesCreatePluginRegistryWithoutArguments` | `static_examples_test.go` (root) | Source scan. |
| `TestExamplePluginsDoNotLogResourceOrEventDetails` | `static_examples_test.go` (root) | Source scan; that no secret reaches events or the standard streams is covered behaviourally by `TestEventsHoldNoSecret` and `TestPluginEventsHoldNoSecret`. |
| `TestExampleSecretFieldsAreSensitive` | `static_examples_test.go` (root) | Source scan of field types; secrets staying masked is covered behaviourally by the `…HoldNoSecret` and `…MasksSecret` tests. |
| `TestExampleSecretFieldScanFindsTheKnownFields` | `static_examples_test.go` (root) | Tested the source scan above. |
| `TestExamplesUsingSecretsCallReveal` | `static_examples_test.go` (root) | Source scan; `TestEnvFunctionReadsSensitiveValue` covers Reveal returning the real value. |
