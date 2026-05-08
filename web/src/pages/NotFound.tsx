import { Link } from "react-router-dom";
import { Home } from "lucide-react";
import { Button } from "@/components/ui/button";

export function NotFound() {
  return (
    <div className="grid min-h-full place-items-center text-center">
      <div className="space-y-4">
        <p className="font-mono text-6xl font-semibold tracking-tight text-primary">404</p>
        <h1 className="text-xl font-semibold">Page not found</h1>
        <Button asChild>
          <Link to="/">
            <Home className="h-4 w-4" />
            Back to dashboard
          </Link>
        </Button>
      </div>
    </div>
  );
}
