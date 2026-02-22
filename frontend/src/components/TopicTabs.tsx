import { Link, useParams, useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";

export function TopicTabs() {
  const { clusterName, topicName } = useParams();
  const location = useLocation();
  const basePath = `/clusters/${clusterName}/topics/${topicName}`;
  const isMessages = location.pathname.endsWith("/messages");

  return (
    <div className="flex gap-1 border-b mb-6">
      <Link
        to={basePath}
        className={cn(
          "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
          !isMessages
            ? "border-primary text-primary"
            : "border-transparent text-muted-foreground hover:text-foreground"
        )}
      >
        Details
      </Link>
      <Link
        to={`${basePath}/messages`}
        className={cn(
          "px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors",
          isMessages
            ? "border-primary text-primary"
            : "border-transparent text-muted-foreground hover:text-foreground"
        )}
      >
        Messages
      </Link>
    </div>
  );
}
