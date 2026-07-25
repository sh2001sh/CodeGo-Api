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
import { useState } from 'react'
import {
  CardStaggerContainer,
  CardStaggerItem,
  FadeIn,
} from '@/components/page-transition'
import { RedemptionCodePanel } from '@/features/wallet/components/redemption-code-panel'
import { useRedemption } from '@/features/wallet/hooks/use-redemption'
import { useTopupInfo } from '@/features/wallet/hooks/use-topup-info'
import { AnnouncementsPanel } from './announcements-panel'
import { FAQPanel } from './faq-panel'
import { OverviewHealthPanel } from './overview-health-panel'
import { OverviewHeroPanel } from './overview-hero-panel'
import { SummaryCards } from './summary-cards'

export function OverviewDashboard() {
  const [redemptionCode, setRedemptionCode] = useState('')
  const { topupInfo } = useTopupInfo()
  const { redeeming, redeemCode } = useRedemption()

  const handleRedeem = async () => {
    const success = await redeemCode(redemptionCode)
    if (success) setRedemptionCode('')
  }

  return (
    <div className='flex flex-col gap-5'>
      <FadeIn>
        <OverviewHeroPanel />
      </FadeIn>

      <CardStaggerContainer className='flex min-w-0 flex-col gap-5'>
        <CardStaggerItem>
          <SummaryCards />
        </CardStaggerItem>

        <CardStaggerItem>
          <div className='grid gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]'>
            <OverviewHealthPanel />
          </div>
        </CardStaggerItem>

        <CardStaggerItem>
          <div className='grid gap-5 lg:grid-cols-2'>
            <AnnouncementsPanel />
            <div className='flex flex-col gap-5'>
              <RedemptionCodePanel
                compact
                title='兑换码'
                description='充值码、套餐码或活动码可直接在这里兑换。'
                topupLink={topupInfo?.topup_link}
                redemptionCode={redemptionCode}
                onRedemptionCodeChange={setRedemptionCode}
                onRedeem={() => void handleRedeem()}
                redeeming={redeeming}
              />
              <FAQPanel />
            </div>
          </div>
        </CardStaggerItem>
      </CardStaggerContainer>
    </div>
  )
}
