"use client";

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { dictionaries, type Language } from "@/lib/content";

export function Faq({ language }: { language: Language }) {
  return (
    <Accordion type="single" collapsible className="w-full">
      {dictionaries[language].faq.items.map((item, index) => (
        <AccordionItem key={item.q} value={String(index)}>
          <AccordionTrigger className="py-6 text-base">
            {item.q}
          </AccordionTrigger>
          <AccordionContent className="max-w-2xl text-base leading-7 text-muted-foreground">
            {item.a}
          </AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}
