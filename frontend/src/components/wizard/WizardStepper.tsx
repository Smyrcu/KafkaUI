import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

interface WizardStepperProps {
  steps: string[];
  currentStep: number;
  onStepClick: (step: number) => void;
}

export function WizardStepper({ steps, currentStep, onStepClick }: WizardStepperProps) {
  return (
    <div className="space-y-3">
      {/* Step indicators */}
      <div className="flex items-center justify-center gap-1.5">
        {steps.map((label, i) => {
          const isCompleted = i < currentStep;
          const isActive = i === currentStep;
          const isPending = i > currentStep;

          return (
            <div key={label} className="flex items-center gap-1.5">
              {i > 0 && (
                <div className={cn("h-px w-6", isCompleted ? "bg-primary" : "bg-border")} />
              )}
              <button
                type="button"
                onClick={() => isCompleted && onStepClick(i)}
                disabled={isPending}
                title={label}
                className={cn(
                  "flex h-8 w-8 items-center justify-center rounded-full text-xs font-bold border-2 transition-all",
                  isCompleted && "bg-primary border-primary text-primary-foreground cursor-pointer hover:opacity-80",
                  isActive && "border-primary text-primary scale-110",
                  isPending && "border-muted-foreground/30 text-muted-foreground cursor-default"
                )}
              >
                {isCompleted ? <Check className="h-3.5 w-3.5" /> : i + 1}
              </button>
            </div>
          );
        })}
      </div>

      {/* Current step label */}
      <p className="text-center text-sm text-muted-foreground">
        Step {currentStep + 1} of {steps.length}: <span className="font-medium text-foreground">{steps[currentStep]}</span>
      </p>
    </div>
  );
}
