export interface UIInput {
  name: string
  label: string
  type: string
  default?: unknown
  options?: string[]
  required?: boolean
  placeholder?: string
}

export interface UIAction {
  name: string
  label: string
  style?: string
}

export interface UIOutputField {
  name: string
  label: string
  type: string
}

export interface UISchema {
  type: string
  inputs?: UIInput[]
  actions?: UIAction[]
  output?: UIOutputField[]
}

export interface WebManifest {
  start_command: string
  port: number
  health_check?: string
  auto_open?: boolean
}

export interface TerminalManifest {
  command: string
  args?: { name: string; label: string; type: string; default?: unknown }[]
}

export interface FileManifest {
  command: string
  input_type: string
  input_extensions?: string[]
  output_type: string
  args?: { name: string; label: string; type: string; default?: unknown }[]
}

export interface ToolManifest {
  api_version?: string
  display_name: string
  description?: string
  icon?: string
  category?: string
  contribution: string
  web?: WebManifest
  terminal?: TerminalManifest
  file?: FileManifest
  ui?: UISchema
}

export interface RuntimeInfo {
  name: string
  version?: string
  available: boolean
  path?: string
  error?: string
  hint?: string
}

// Matches model.ToolInfo from the backend.
export interface MarcusToolInfo {
  id: string
  name: string
  display_name: string
  description?: string
  icon?: string
  category: string
  version?: string
  source: string
  contribution: string
  package_path?: string
  manifest: string
  entry_point?: string
  enabled: boolean
  healthy?: boolean
  health_error?: string
  health_hint?: string
  last_seen: string
  created_at: string
}
