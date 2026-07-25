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
import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { ModelsDialogs } from './components/models-dialogs'
import { ModelsPrimaryButtons } from './components/models-primary-buttons'
import { ModelsProvider } from './components/models-provider'
import { ModelsTable } from './components/models-table'

function ModelsContent() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Models')}</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t('Maintain the model catalog used by your API routes.')}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <ModelsPrimaryButtons />
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <ModelsTable />
      </SectionPageLayout.Content>
      <ModelsDialogs />
    </SectionPageLayout>
  )
}

export function Models() {
  return (
    <ModelsProvider>
      <ModelsContent />
    </ModelsProvider>
  )
}
