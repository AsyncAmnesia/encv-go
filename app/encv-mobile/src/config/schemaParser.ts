import schemaData from '@/config/schema.json'

export type FieldType = 'string' | 'integer' | 'boolean' | 'object' | 'array'

export interface FieldDef {
  key: string
  label: string
  description: string
  type: FieldType
  required: boolean
  default?: unknown
  enum?: string[]
  properties?: FieldDef[]
  isPassword?: boolean
  sectionTitle?: string
  isMap?: boolean
  mapItemFields?: FieldDef[]
  isPath?: boolean
}

interface JsonSchemaProperty {
  type?: string
  description?: string
  $ref?: string
  properties?: Record<string, JsonSchemaProperty>
  required?: string[]
  additionalProperties?: boolean | JsonSchemaProperty
  items?: JsonSchemaProperty
  enum?: string[]
  default?: unknown
}

interface JsonSchemaDef {
  $defs?: Record<string, JsonSchemaProperty>
  $ref?: string
  properties?: Record<string, JsonSchemaProperty>
  required?: string[]
  description?: string
}

const defs = (schemaData as JsonSchemaDef).$defs || {}

function resolveRef(ref: string): JsonSchemaProperty {
  const match = ref.match(/^#\/\$defs\/(.+)$/)
  if (match && defs[match[1]]) {
    return defs[match[1]]
  }
  return {}
}

function extractSectionTitle(desc: string): { sectionTitle: string; cleanDesc: string } {
  const match = desc.match(/^---\s*(.+?)\s*---\s*\n?/)
  if (match) {
    return {
      sectionTitle: match[1].trim(),
      cleanDesc: desc.slice(match[0].length).trim(),
    }
  }
  return { sectionTitle: '', cleanDesc: desc }
}

function parseProperty(
  key: string,
  prop: JsonSchemaProperty,
  parentRequired: string[] = [],
): FieldDef {
  let resolved = prop
  if (prop.$ref) {
    resolved = { ...resolveRef(prop.$ref), description: prop.description || resolveRef(prop.$ref).description }
  }

  const isRequired = parentRequired.includes(key)
  const description = resolved.description || ''
  const { sectionTitle, cleanDesc } = extractSectionTitle(description)

  const field: FieldDef = {
    key,
    label: formatLabel(key),
    description: cleanDesc,
    type: (resolved.type || 'string') as FieldType,
    required: isRequired,
    sectionTitle: sectionTitle || undefined,
    isPassword: isPasswordField(key),
    isPath: isPathField(key),
  }

  if (resolved.enum) {
    field.enum = resolved.enum
  }

  if (resolved.type === 'object' && resolved.properties) {
    const childRequired = resolved.required || []
    field.properties = Object.entries(resolved.properties).map(([childKey, childProp]) =>
      parseProperty(childKey, childProp, childRequired),
    )
  }

  if (resolved.type === 'object' && resolved.additionalProperties && typeof resolved.additionalProperties === 'object') {
    if (resolved.additionalProperties.$ref) {
      const itemDef = resolveRef(resolved.additionalProperties.$ref)
      if (itemDef.properties) {
        const childRequired = itemDef.required || []
        field.isMap = true
        field.mapItemFields = Object.entries(itemDef.properties).map(([childKey, childProp]) =>
          parseProperty(childKey, childProp, childRequired),
        )
      }
    } else {
      field.isMap = true
    }
  }

  return field
}

function formatLabel(key: string): string {
  return key
    .replace(/_/g, ' ')
    .replace(/\b\w/g, c => c.toUpperCase())
}

function isPasswordField(key: string): boolean {
  return key.toLowerCase().includes('password')
}

function isPathField(key: string): boolean {
  const pathKeys = ['output_path', 'dir', 'file', 'plugin_cache_dir', 'root']
  return pathKeys.includes(key) || key.includes('_path') || key.includes('_dir')
}

export function parseSchema(): FieldDef[] {
  const root = schemaData as JsonSchemaDef
  const rootRequired = root.required || []
  const rootProperties = root.properties || {}

  const configDef = root.$ref ? resolveRef(root.$ref) : root
  const properties = configDef.properties || rootProperties
  const required = configDef.required || rootRequired

  return Object.entries(properties)
    .filter(([key]) => key !== '$schema')
    .map(([key, prop]) => parseProperty(key, prop, required))
}

export function getDefaultValue(field: FieldDef): unknown {
  switch (field.type) {
    case 'boolean':
      return false
    case 'integer':
      return 0
    case 'string':
      return ''
    case 'object':
      if (field.properties) {
        const obj: Record<string, unknown> = {}
        for (const child of field.properties) {
          obj[child.key] = getDefaultValue(child)
        }
        return obj
      }
      return {}
    case 'array':
      return []
    default:
      return ''
  }
}
