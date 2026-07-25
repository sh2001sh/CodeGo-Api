/*
Copyright (C) 2026 codego-api contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, Wrench } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SectionPageLayout } from '@/components/layout'

type Reconciliation = {
  account_id: string
  consistent: boolean
  actual: { available_balance: number; reserved_balance: number }
  expected: { available_balance: number; reserved_balance: number }
}

type PerfSummary = {
  data?: {
    models?: Array<{
      model_name: string
      success_rate: number
      avg_latency_ms: number
      request_count?: number
    }>
  }
}

export function Operations() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const reconciliations = useQuery({
    queryKey: ['billing-reconciliations'],
    queryFn: async () =>
      (
        await api.get<{ data: Reconciliation[] }>(
          '/api/billing/reconciliations'
        )
      ).data.data ?? [],
  })
  const metrics = useQuery({
    queryKey: ['operations-slo'],
    queryFn: async () =>
      (
        await api.get<PerfSummary>('/api/perf-metrics/summary', {
          params: { hours: 24 },
        })
      ).data.data?.models ?? [],
  })
  const repair = useMutation({
    mutationFn: async (accountID: string) =>
      api.post(`/api/billing/reconciliations/${accountID}/repair`),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['billing-reconciliations'] }),
  })
  const items = reconciliations.data ?? []
  const inconsistent = items.filter((item) => !item.consistent)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Operations')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Monitor routing reliability and ledger integrity')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          onClick={() => {
            reconciliations.refetch()
            metrics.refetch()
          }}
          disabled={reconciliations.isFetching || metrics.isFetching}
        >
          <RefreshCw />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-2'>
            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('Ledger integrity')}
                </CardTitle>
              </CardHeader>
              <CardContent className='space-y-1'>
                <div className='text-2xl font-semibold tabular-nums'>
                  {reconciliations.isLoading ? (
                    <Skeleton className='h-8 w-16' />
                  ) : (
                    `${items.length - inconsistent.length} / ${items.length}`
                  )}
                </div>
                <p className='text-muted-foreground text-sm'>
                  {t('Accounts matching their ledger reconstruction')}
                </p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('24h routing SLO')}
                </CardTitle>
              </CardHeader>
              <CardContent className='space-y-1'>
                <div className='text-2xl font-semibold tabular-nums'>
                  {metrics.isLoading ? (
                    <Skeleton className='h-8 w-16' />
                  ) : (
                    `${metrics.data?.length ?? 0}`
                  )}
                </div>
                <p className='text-muted-foreground text-sm'>
                  {t('Models with recorded reliability samples')}
                </p>
              </CardContent>
            </Card>
          </div>
          <Card>
            <CardHeader>
              <CardTitle className='text-base'>
                {t('Ledger reconciliation')}
              </CardTitle>
            </CardHeader>
            <CardContent className='p-0'>
              {reconciliations.isError ? (
                <p className='text-destructive p-6 text-sm'>
                  {t('Unable to load ledger reconciliation')}
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Account')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Available')}</TableHead>
                      <TableHead>{t('Reserved')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Action')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {items.map((item) => (
                      <TableRow key={item.account_id}>
                        <TableCell className='font-mono text-xs'>
                          {item.account_id}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              item.consistent ? 'secondary' : 'destructive'
                            }
                          >
                            {item.consistent ? t('Consistent') : t('Mismatch')}
                          </Badge>
                        </TableCell>
                        <TableCell className='tabular-nums'>
                          {item.actual.available_balance}
                        </TableCell>
                        <TableCell className='tabular-nums'>
                          {item.actual.reserved_balance}
                        </TableCell>
                        <TableCell className='text-right'>
                          {!item.consistent && (
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={repair.isPending}
                              onClick={() => repair.mutate(item.account_id)}
                            >
                              <Wrench />
                              {t('Repair')}
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                    {!reconciliations.isLoading && items.length === 0 && (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('No ledger accounts to reconcile')}
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
