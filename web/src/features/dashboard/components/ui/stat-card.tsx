import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Card, CardContent } from '@/components/ui/card'

export function StatCard(props: { title:string; value:ReactNode; description?:ReactNode; icon?:LucideIcon; tone?:string; sparkline?:number[]; sparklineVariant?:string; loading?:boolean; compactMobile?:boolean }) {
  const Icon = props.icon
  return <Card><CardContent className='p-3'><div className='text-muted-foreground text-xs'>{props.title}</div><div className='flex items-center gap-2 text-lg font-semibold'>{Icon && <Icon className='size-4'/>}{props.loading ? '—' : props.value}</div>{props.description && <div className='text-muted-foreground text-xs'>{props.description}</div>}</CardContent></Card>
}
