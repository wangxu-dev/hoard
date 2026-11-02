import { Card } from "@app-ui/card";
import { X, Minus, Maximize2 } from "lucide-react";
import { DemoFrame } from "@/components/demo-frame";
import type { Language } from "@/lib/content";

export function LibraryPreview({ language }: { language: Language }) {
  return (
    <div className="app-window-wrap" data-testid="app-window">
      <Card className="gap-0 overflow-hidden py-0 shadow-lg">
        <div
          className="group flex h-10 shrink-0 items-center gap-2 border-b bg-sidebar px-4"
          aria-hidden="true"
        >
          {[
            { icon: X, color: "bg-window-close" },
            { icon: Minus, color: "bg-window-minimize" },
            { icon: Maximize2, color: "bg-window-maximize" },
          ].map(({ icon: Icon, color }) => (
            <span
              key={color}
              className={`flex size-3 items-center justify-center rounded-full ${color}`}
            >
              <Icon
                className="size-2 text-window-control opacity-0 group-hover:opacity-70"
                strokeWidth={2.5}
              />
            </span>
          ))}
        </div>
        <div className="app-demo-viewport">
          <DemoFrame
            language={language}
            className="app-demo-content border-0 bg-background"
          />
        </div>
      </Card>
    </div>
  );
}
