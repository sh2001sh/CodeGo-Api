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
import { getPrivacyPolicy } from './api'
import { LegalDocument } from './legal-document'

export function PrivacyPolicy() {
  const { t } = useTranslation()

  return (
    <LegalDocument
      title={t('Privacy Policy')}
      queryKey='privacy-policy'
      fetchDocument={getPrivacyPolicy}
      emptyMessage={t(
        'The administrator has not configured a privacy policy yet.'
      )}
      fallbackContent={t(`# codego-api 隐私政策

codego-api 只处理提供账号认证、API 请求、路由、计费和运行审计所需的信息。可能涉及账号标识、访问时间、请求元数据、额度统计、错误信息以及部署方启用的安全记录。

API 请求会按部署配置转发到所选择的 provider。部署者应根据自己的场景配置日志保留期限、访问权限、数据库和第三方服务，并避免在公开环境中保存不必要的秘密或个人信息。

你可以通过部署管理员了解数据保留和删除规则。第三方 provider 的数据处理受其自身政策约束。`)}
    />
  )
}
