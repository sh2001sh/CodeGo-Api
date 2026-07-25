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
export interface ImageWorkspaceItem {
  id: number
  session_id: string
  batch_id: string
  source_item_id: number
  model: string
  prompt: string
  revised_prompt: string
  image_index: number
  status: 'ready' | 'expired' | 'failed'
  image_url: string
  download_url: string
  original_url: string
  expires_at: number
  created_at: number
  error_message: string
}

export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}

export type ImageWorkspaceMode = 'generate' | 'edit'

export interface ImageWorkspaceFormState {
  mode: ImageWorkspaceMode
  model: string
  group: string
  prompt: string
  size: string
  quality: string
  count: string
}
