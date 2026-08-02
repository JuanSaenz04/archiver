import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";
const variants = cva(
	"inline-flex items-center rounded-full px-2 py-0.5 text-[.68rem] font-bold uppercase tracking-wider transition-colors duration-200",
	{
		variants: {
			status: {
				pending: "bg-warning/20 text-foreground",
				running: "bg-info/20 text-foreground",
				completed: "bg-success/20 text-foreground",
				failed: "bg-destructive/20 text-destructive",
			},
		},
		defaultVariants: { status: "running" },
	},
);
export function StatusPill({
	status,
	className,
}: {
	status: string;
	className?: string;
}) {
	const normalized = (
		["pending", "running", "completed", "failed"] as const
	).includes(status as "pending")
		? (status as VariantProps<typeof variants>["status"])
		: "running";
	return (
		<span className={cn(variants({ status: normalized }), className)}>
			{status}
		</span>
	);
}
