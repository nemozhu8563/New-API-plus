import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useApiInfo } from '../../hooks/use-status-data'

export function ApiInfoPanel() {
  const { items } = useApiInfo()
  if (!items.length) return null
  return <Card><CardHeader><CardTitle>API</CardTitle></CardHeader><CardContent>{items.map((item, i) => <div key={i}>{item.url}</div>)}</CardContent></Card>
}
