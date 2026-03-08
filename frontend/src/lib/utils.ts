import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function rowClassName(index: number) {
  return index % 2 === 1 ? "bg-muted/30" : "";
}
