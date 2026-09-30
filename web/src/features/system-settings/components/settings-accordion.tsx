import {
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from '@/components/ui/accordion'
import { cn } from '@/lib/utils'

type SettingsAccordionProps = {
  value: string
  title: string
  children: React.ReactNode
  className?: string
  keepMounted?: boolean
}

export function SettingsAccordion({
  value,
  title,
  children,
  className,
  keepMounted,
}: SettingsAccordionProps) {
  return (
    <AccordionItem value={value} className={cn(className)}>
      <AccordionTrigger className='hover:no-underline'>
        <div className='flex flex-col gap-1 text-left'>
          <div className='text-base font-semibold'>{title}</div>
        </div>
      </AccordionTrigger>
      <AccordionContent className='pt-4' keepMounted={keepMounted}>
        {children}
      </AccordionContent>
    </AccordionItem>
  )
}
