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
import type { RoutePoolDetail } from '../types'

type RoutePoolListProps = {
  pools: RoutePoolDetail[]
  loading: boolean
  onEdit: (detail: RoutePoolDetail) => void
}

export function RoutePoolList({ pools, loading, onEdit }: RoutePoolListProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>已配置分组</CardTitle>
      </CardHeader>
      <CardContent className='p-0'>
        {loading ? (
          <Skeleton className='m-4 h-28' />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>分组</TableHead>
                <TableHead>名称</TableHead>
                <TableHead>成员</TableHead>
                <TableHead>状态</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {pools.map((detail) => (
                <TableRow key={detail.pool.id}>
                  <TableCell className='font-mono'>
                    {detail.pool.group}
                  </TableCell>
                  <TableCell>{detail.pool.name}</TableCell>
                  <TableCell>{detail.members.length}</TableCell>
                  <TableCell>
                    <Badge
                      variant={detail.pool.enabled ? 'secondary' : 'outline'}
                    >
                      {detail.pool.enabled ? '启用' : '停用'}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-right'>
                    <Button
                      size='sm'
                      variant='outline'
                      onClick={() => onEdit(detail)}
                    >
                      编辑
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {pools.length === 0 && (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-muted-foreground h-24 text-center'
                  >
                    尚未配置自动池。
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
