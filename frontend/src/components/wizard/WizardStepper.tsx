import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

interface WizardStepperProps {
  steps: string[];
  currentStep: number;
  onStepClick: (step: number) => void;
}

export function WizardStepper({ steps, currentStep, onStepClick }: WizardStepperProps) {
  return (
    <div className="flex items-center gap-2 mb-6">
      {steps.map((label, i) => {
        const isCompleted = i < currentStep;
        const isActive = i === currentStep;
        const isPending = i > currentStep;

        return (
          <div key={label} className="flex items-center gap-2">
            {i > 0 && (
              <div className={cn("h-px w-8", isCompleted ? "bg-primary" : "bg-border")} />
            )}
            <button
              type="button"
              onClick={() => isCompleted && onStepClick(i)}
              disabled={isPending}
              className={cn(
                "flex items-center gap-2 text-sm font-medium transition-colors",
                isCompleted && "text-primary cursor-pointer hover:underline",
                isActive && "text-foreground",
                isPending && "text-muted-foreground cursor-default"
              )}
            >
              <div
                className={cn(
                  "flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold border-2 transition-colors",
                  isCompleted && "bg-primary border-primary text-primary-foreground",
                  isActive && "border-primary text-primary",
                  isPending && "border-muted-foreground/30 text-muted-foreground"
                )}
              >
                {isCompleted ? <Check className="h-4 w-4" /> : i + 1}
              </div>
              <span className="hidden sm:inline">{label}</span>
            </button>
          </div>
        );
      })}
    </div>
  );
}
