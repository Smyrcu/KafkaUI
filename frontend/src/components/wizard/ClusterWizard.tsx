import { useReducer } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { api, type AddClusterRequest, type TestConnectionResult } from "@/lib/api";
import { WizardStepper } from "./WizardStepper";
import { ConnectionStep, validateBootstrap, validateName } from "./steps/ConnectionStep";
import { SecurityStep } from "./steps/SecurityStep";
import { AuthStep } from "./steps/AuthStep";
import { SchemaRegistryStep } from "./steps/SchemaRegistryStep";
import { KafkaConnectStep } from "./steps/KafkaConnectStep";
import { KsqlStep } from "./steps/KsqlStep";
import { ReviewStep } from "./steps/ReviewStep";

const STEPS = ["Connection", "Security", "Auth", "Schema Registry", "Kafka Connect", "KSQL", "Review"];
const REQUIRED_STEPS = new Set([0, 6]); // Connection and Review

interface WizardState {
  step: number;
  data: AddClusterRequest;
  testResult: TestConnectionResult | null;
  testing: boolean;
  saving: boolean;
  error: string | null;
}

type Action =
  | { type: "SET_STEP"; step: number }
  | { type: "UPDATE_DATA"; data: Partial<AddClusterRequest> }
  | { type: "SET_TEST_RESULT"; result: TestConnectionResult | null }
  | { type: "SET_TESTING"; testing: boolean }
  | { type: "SET_SAVING"; saving: boolean }
  | { type: "SET_ERROR"; error: string | null };

function reducer(state: WizardState, action: Action): WizardState {
  switch (action.type) {
    case "SET_STEP":
      return { ...state, step: action.step };
    case "UPDATE_DATA":
      return { ...state, data: { ...state.data, ...action.data }, testResult: null };
    case "SET_TEST_RESULT":
      return { ...state, testResult: action.result };
    case "SET_TESTING":
      return { ...state, testing: action.testing };
    case "SET_SAVING":
      return { ...state, saving: action.saving };
    case "SET_ERROR":
      return { ...state, error: action.error };
  }
}

interface ClusterWizardProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  initialData?: AddClusterRequest;
}

export function ClusterWizard({ open, onClose, onSaved, initialData }: ClusterWizardProps) {
  const [state, dispatch] = useReducer(reducer, {
    step: 0,
    data: initialData ?? { name: "", bootstrapServers: "" },
    testResult: null,
    testing: false,
    saving: false,
    error: null,
  });

  const { step, data, testResult, testing, saving, error } = state;
  const isEdit = !!initialData;
  const canNext = step === 0
    ? data.name.trim() !== "" && data.bootstrapServers.trim() !== "" && !validateName(data.name) && !validateBootstrap(data.bootstrapServers)
    : true;
  const isOptionalStep = !REQUIRED_STEPS.has(step);
  const isLastStep = step === STEPS.length - 1;

  const handleTest = async () => {
    dispatch({ type: "SET_TESTING", testing: true });
    dispatch({ type: "SET_TEST_RESULT", result: null });
    try {
      const result = await api.admin.testConnection(data);
      dispatch({ type: "SET_TEST_RESULT", result });
    } catch (e) {
      dispatch({ type: "SET_TEST_RESULT", result: { status: "error", error: String(e) } });
    } finally {
      dispatch({ type: "SET_TESTING", testing: false });
    }
  };

  const handleSave = async () => {
    dispatch({ type: "SET_SAVING", saving: true });
    dispatch({ type: "SET_ERROR", error: null });
    try {
      if (isEdit) {
        await api.admin.updateCluster(data.name, data, false);
      } else {
        await api.admin.addCluster(data, false);
      }
      onSaved();
    } catch (e) {
      dispatch({ type: "SET_ERROR", error: e instanceof Error ? e.message : String(e) });
    } finally {
      dispatch({ type: "SET_SAVING", saving: false });
    }
  };

  const handleNext = () => {
    if (!isLastStep && canNext) {
      dispatch({ type: "SET_STEP", step: step + 1 });
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !isLastStep && canNext) {
      e.preventDefault();
      handleNext();
    }
  };

  const renderStep = () => {
    switch (step) {
      case 0:
        return (
          <ConnectionStep
            name={data.name}
            bootstrapServers={data.bootstrapServers}
            onChange={(d) => dispatch({ type: "UPDATE_DATA", data: d })}
          />
        );
      case 1:
        return <SecurityStep tls={data.tls} onChange={(tls) => dispatch({ type: "UPDATE_DATA", data: { tls } })} />;
      case 2:
        return <AuthStep sasl={data.sasl} onChange={(sasl) => dispatch({ type: "UPDATE_DATA", data: { sasl } })} />;
      case 3:
        return <SchemaRegistryStep schemaRegistry={data.schemaRegistry} onChange={(sr) => dispatch({ type: "UPDATE_DATA", data: { schemaRegistry: sr } })} />;
      case 4:
        return <KafkaConnectStep kafkaConnect={data.kafkaConnect} onChange={(kc) => dispatch({ type: "UPDATE_DATA", data: { kafkaConnect: kc } })} />;
      case 5:
        return <KsqlStep ksql={data.ksql} onChange={(ksql) => dispatch({ type: "UPDATE_DATA", data: { ksql } })} />;
      case 6:
        return <ReviewStep data={data} testResult={testResult} testing={testing} onTest={handleTest} />;
      default:
        return null;
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-lg" onKeyDown={handleKeyDown}>
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Cluster" : "Add Cluster"}</DialogTitle>
        </DialogHeader>

        <WizardStepper steps={STEPS} currentStep={step} onStepClick={(s) => dispatch({ type: "SET_STEP", step: s })} />

        <div className="min-h-[180px] py-4">
          {renderStep()}
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}

        <div className="flex justify-between pt-2 border-t">
          <div>
            {step > 0 && (
              <Button variant="outline" onClick={() => dispatch({ type: "SET_STEP", step: step - 1 })}>
                Back
              </Button>
            )}
          </div>
          <div className="flex gap-2">
            {isOptionalStep && !isLastStep && (
              <Button variant="ghost" onClick={handleNext}>
                Skip
              </Button>
            )}
            {!isLastStep && (
              <Button onClick={handleNext} disabled={!canNext}>
                Next
              </Button>
            )}
            {isLastStep && (
              <Button
                onClick={() => {
                  if (!testResult && !window.confirm("Connection was not tested. Save anyway?")) return;
                  handleSave();
                }}
                disabled={saving}
              >
                {saving ? "Saving..." : "Save"}
              </Button>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
