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
import { getUserAgreement } from './api'
import { LegalDocument } from './legal-document'

export function UserAgreement() {
  const { t } = useTranslation()

  return (
    <LegalDocument
      title={t('User Agreement')}
      queryKey='user-agreement'
      fetchDocument={getUserAgreement}
      emptyMessage={t(
        'The administrator has not configured a user agreement yet.'
      )}
      fallbackContent={t(`# codego-api 用户协议

codego-api 是用于统一管理 API provider、模型路由、API key、额度和使用记录的软件平台。

你应当遵守适用的法律法规、第三方 provider 的服务条款以及部署组织的安全规范。不得利用平台绕过权限、攻击系统、滥用上游服务、传播违法内容或干扰其他用户。

你负责保护自己的账号、API key 和部署凭据。平台管理员可以根据安全、合规和维护需要调整配置、限制访问或暂停服务。

模型输出由上游 provider 生成，使用者应自行评估其准确性、合法性和适用性。除法律明确要求外，软件按现状提供，不对特定用途作保证。`)}
    />
  )
}
