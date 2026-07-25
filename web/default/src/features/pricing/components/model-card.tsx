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
import { memo } from 'react'
import { ChevronRight, Copy, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Badge } from '@/components/ui/badge'
import { StatusBadge } from '@/components/status-badge'
import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { getFreeEligibleGroups, isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'
import { ModelPerfBadge, type ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  perf?: ModelPerfBadgeData
  groupRatios?: Record<string, number>
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const tags = parseTags(props.model.tags)
  const groups = props.model.enable_groups || []
  const endpoints = props.model.supported_endpoint_types || []
  const vendorIcon = props.model.vendor_icon
    ? getLobeIcon(props.model.vendor_icon, 28)
    : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const hasCachedPrice = isTokenBased && props.model.cache_ratio != null
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(props.model),
      })
    : null

  const primaryGroup = groups[0]
  const bottomTags = [...endpoints.slice(0, 2), ...tags.slice(0, 2)]
  const hiddenCount =
    Math.max(groups.length - 1, 0) +
    Math.max(endpoints.length - 2, 0) +
    Math.max(tags.length - 2, 0)
  const freeGroups = getFreeEligibleGroups(props.model, props.groupRatios)
  const isFreeModel = freeGroups.length > 0

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  return (
    <div
      className={cn(
        'group bg-card relative flex min-h-[230px] flex-col overflow-hidden rounded-2xl border p-3 transition-all duration-200 sm:p-5',
        isFreeModel
          ? 'border-success/25 hover:border-success/45 shadow-[0_0_18px_color-mix(in_oklch,var(--success)_6%,transparent)] hover:shadow-[0_0_22px_color-mix(in_oklch,var(--success)_14%,transparent)]'
          : 'hover:border-foreground/18 hover:bg-card'
      )}
    >
      {isFreeModel && (
        <div className='pointer-events-none absolute right-4 bottom-4 z-0'>
          <Badge className='border-success/15 bg-success/8 text-success/55 rounded-full border px-3 py-1 text-[11px] tracking-[0.16em] uppercase shadow-none'>
            <Sparkles className='size-3' />
            {t('Free')}
          </Badge>
        </div>
      )}

      {/* Header: reserve the full content width for long model identifiers. */}
      <div className='relative z-10'>
        <div className='flex min-w-0 items-start gap-2.5 sm:gap-3'>
          <div className='bg-muted/45 ring-border/60 flex size-9 shrink-0 items-center justify-center rounded-xl ring-1 sm:size-10'>
            {vendorIcon || (
              <span className='text-muted-foreground text-sm font-bold'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0 flex-1'>
            <h3
              className='text-foreground min-w-0 font-mono text-[15px] leading-[1.35] font-semibold break-all'
              title={props.model.model_name}
            >
              {props.model.model_name}
            </h3>

            <div className='mt-0.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-xs sm:mt-1 sm:gap-x-3'>
              {dynamicSummary ? (
                dynamicSummary.isSpecialExpression ? (
                  <span className='min-w-0'>
                    <span className='text-warning'>
                      {t('Special billing expression')}
                    </span>
                    <code className='text-muted-foreground/70 mt-0.5 line-clamp-1 block font-mono text-[11px] break-all'>
                      {dynamicSummary.rawExpression}
                    </code>
                  </span>
                ) : dynamicSummary.primaryEntries.length > 0 ? (
                  <>
                    {dynamicSummary.primaryEntries.map((entry) => (
                      <span
                        key={entry.key}
                        className='text-muted-foreground whitespace-nowrap'
                      >
                        {t(entry.shortLabel)}{' '}
                        <span className='text-foreground font-mono font-semibold'>
                          {entry.formatted}
                        </span>
                        /{tokenUnitLabel}
                      </span>
                    ))}
                  </>
                ) : (
                  <span className='text-muted-foreground text-xs'>
                    {t('Dynamic Pricing')}
                  </span>
                )
              ) : isTokenBased ? (
                <>
                  <span className='text-muted-foreground whitespace-nowrap'>
                    {t('Input')}{' '}
                    <span className='text-foreground font-mono font-semibold'>
                      {formatPrice(
                        props.model,
                        'input',
                        tokenUnit,
                        showRechargePrice,
                        priceRate,
                        usdExchangeRate
                      )}
                    </span>
                    /{tokenUnitLabel}
                  </span>
                  <span className='text-muted-foreground whitespace-nowrap'>
                    {t('Output')}{' '}
                    <span className='text-foreground font-mono font-semibold'>
                      {formatPrice(
                        props.model,
                        'output',
                        tokenUnit,
                        showRechargePrice,
                        priceRate,
                        usdExchangeRate
                      )}
                    </span>
                    /{tokenUnitLabel}
                  </span>
                  {hasCachedPrice && (
                    <span className='text-muted-foreground/60 whitespace-nowrap'>
                      {t('Cached')}{' '}
                      <span className='font-mono'>
                        {formatPrice(
                          props.model,
                          'cache',
                          tokenUnit,
                          showRechargePrice,
                          priceRate,
                          usdExchangeRate
                        )}
                      </span>
                    </span>
                  )}
                </>
              ) : (
                <span className='text-muted-foreground whitespace-nowrap'>
                  <span className='text-foreground font-mono font-semibold'>
                    {formatRequestPrice(
                      props.model,
                      showRechargePrice,
                      priceRate,
                      usdExchangeRate
                    )}
                  </span>{' '}
                  / {t('request')}
                </span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Description */}
      <p className='text-muted-foreground relative z-10 mt-2 line-clamp-1 flex-1 text-[13px] leading-relaxed sm:mt-4 sm:line-clamp-2 sm:min-h-[2.5rem]'>
        {props.model.description || ''}
      </p>

      {/* Footer: left metadata and right performance summary share row alignment */}
      <div className='relative z-10 mt-2 space-y-1 sm:mt-4'>
        <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
          {primaryGroup && (
            <span className='text-muted-foreground bg-muted/45 inline-flex max-w-full shrink-0 items-center rounded-md px-2 py-0.5 text-xs font-medium whitespace-nowrap'>
              {primaryGroup} {t('Groups')}
            </span>
          )}
          <span className='text-muted-foreground bg-muted/45 inline-flex shrink-0 items-center rounded-md px-2 py-0.5 text-xs font-medium whitespace-nowrap'>
            {isTokenBased ? t('Token-based') : t('Per Request')}
          </span>
          {isDynamicPricing && (
            <StatusBadge
              label={t('Dynamic Pricing')}
              variant='warning'
              copyable={false}
              size='sm'
            />
          )}
        </div>

        <div className='max-h-0 min-w-0 overflow-hidden opacity-0 transition-all duration-200 group-focus-within:max-h-20 group-focus-within:opacity-100 group-hover:max-h-20 group-hover:opacity-100'>
          <div className='flex flex-wrap items-center gap-x-2.5 gap-y-1 pt-1 sm:gap-x-3'>
            <ModelPerfBadge perf={props.perf} className='mr-1 opacity-100' />
            {bottomTags.map((item) => (
              <span
                key={item}
                className='text-muted-foreground/70 text-xs whitespace-nowrap'
              >
                {item}
              </span>
            ))}
            <span className='text-muted-foreground/50 text-xs whitespace-nowrap'>
              {tokenUnitLabel}
            </span>
            {hiddenCount > 0 && (
              <span className='text-muted-foreground/40 text-xs whitespace-nowrap'>
                +{hiddenCount}
              </span>
            )}
            {isFreeModel && (
              <span className='text-success text-xs font-medium whitespace-nowrap'>
                {freeGroups[0] ? `${freeGroups[0]} · ` : ''}
                {t('Available at zero group ratio')}
              </span>
            )}
          </div>
        </div>
      </div>

      <div className='border-border/60 relative z-10 mt-3 flex items-center justify-end gap-1.5 border-t pt-3'>
        <button
          type='button'
          onClick={props.onClick}
          className='text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center gap-1 rounded-lg border px-2 py-1 text-xs transition-colors sm:px-2.5 sm:py-1.5'
        >
          {t('Details')}
          <ChevronRight className='size-3.5' />
        </button>
        <button
          type='button'
          onClick={handleCopy}
          className='text-muted-foreground hover:text-foreground hover:bg-muted rounded-lg border p-1.5 transition-colors'
          title={t('Copy')}
        >
          <Copy className='size-3.5' />
        </button>
      </div>
    </div>
  )
})
