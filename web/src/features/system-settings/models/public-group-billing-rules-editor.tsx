import { ChevronDown, ChevronUp, Plus, Trash2 } from 'lucide-react'
/*
Copyright (C) 2023-2026 QuantumNous

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

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const sectionCardClassName =
  'relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border before:border-border/90'
const sectionHeaderClassName = 'border-b bg-muted/20'

let _idCounter = 0
function uid(prefix: string) {
  _idCounter += 1
  return `${prefix}_${_idCounter}`
}

function parseNestedObject<T>(
  value: string
): Record<string, Record<string, T>> {
  if (!value || !value.trim()) return {}
  try {
    const parsed = JSON.parse(value) as unknown
    if (
      typeof parsed !== 'object' ||
      parsed === null ||
      Array.isArray(parsed)
    ) {
      return {}
    }
    return parsed as Record<string, Record<string, T>>
  } catch {
    return {}
  }
}

function serializeNestedObject<T>(
  value: Record<string, Record<string, T>>
): string {
  return Object.keys(value).length === 0 ? '{}' : JSON.stringify(value, null, 2)
}

type TagRatioRule = {
  _id: string
  publicGroup: string
  channelTag: string
  ratio: number
}

type ModelTagRule = {
  _id: string
  publicGroup: string
  modelName: string
  channelTag: string
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function flattenTagRatioRules(value: string): TagRatioRule[] {
  const nested = parseNestedObject<number>(value)
  const rules: TagRatioRule[] = []

  for (const [publicGroup, tagRules] of Object.entries(nested)) {
    if (typeof tagRules !== 'object' || tagRules === null) continue
    for (const [channelTag, ratio] of Object.entries(tagRules)) {
      rules.push({
        _id: uid('pgtr'),
        publicGroup,
        channelTag,
        ratio: normalizeRatio(ratio),
      })
    }
  }

  return rules
}

function flattenModelTagRules(value: string): ModelTagRule[] {
  const nested = parseNestedObject<string>(value)
  const rules: ModelTagRule[] = []

  for (const [publicGroup, modelRules] of Object.entries(nested)) {
    if (typeof modelRules !== 'object' || modelRules === null) continue
    for (const [modelName, channelTag] of Object.entries(modelRules)) {
      rules.push({
        _id: uid('pgmt'),
        publicGroup,
        modelName,
        channelTag: typeof channelTag === 'string' ? channelTag : '',
      })
    }
  }

  return rules
}

function serializeTagRatioRules(rules: TagRatioRule[]): string {
  const nested: Record<string, Record<string, number>> = {}

  for (const rule of rules) {
    const publicGroup = rule.publicGroup.trim()
    const channelTag = rule.channelTag.trim()
    if (!publicGroup || !channelTag) continue
    if (!nested[publicGroup]) nested[publicGroup] = {}
    nested[publicGroup][channelTag] = normalizeRatio(rule.ratio)
  }

  return serializeNestedObject(nested)
}

function serializeModelTagRules(rules: ModelTagRule[]): string {
  const nested: Record<string, Record<string, string>> = {}

  for (const rule of rules) {
    const publicGroup = rule.publicGroup.trim()
    const modelName = rule.modelName.trim()
    const channelTag = rule.channelTag.trim()
    if (!publicGroup || !modelName || !channelTag) continue
    if (!nested[publicGroup]) nested[publicGroup] = {}
    nested[publicGroup][modelName] = channelTag
  }

  return serializeNestedObject(nested)
}

function groupRules<T extends { publicGroup: string }>(rules: T[]) {
  const map: Record<string, T[]> = {}
  const order: string[] = []

  for (const rule of rules) {
    if (!rule.publicGroup) continue
    if (!map[rule.publicGroup]) {
      map[rule.publicGroup] = []
      order.push(rule.publicGroup)
    }
    map[rule.publicGroup].push(rule)
  }

  return order.map((name) => ({ name, items: map[name] }))
}

type GroupPickerProps = {
  datalistId: string
  value: string
  placeholder: string
  groupNames: string[]
  onChange: (value: string) => void
  onAdd: () => void
}

function GroupPicker({
  datalistId,
  value,
  placeholder,
  groupNames,
  onChange,
  onAdd,
}: GroupPickerProps) {
  const { t } = useTranslation()

  return (
    <div className='flex items-center justify-center gap-2 pt-2'>
      <Input
        className='w-[220px]'
        list={datalistId}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            onAdd()
          }
        }}
      />
      <datalist id={datalistId}>
        {groupNames.map((name) => (
          <option key={name} value={name} />
        ))}
      </datalist>
      <Button variant='outline' size='sm' onClick={onAdd}>
        <Plus className='mr-1 h-4 w-4' />
        {t('Add public group rules')}
      </Button>
    </div>
  )
}

type TagRatioGroupSectionProps = {
  groupName: string
  items: TagRatioRule[]
  onUpdate: (
    id: string,
    field: keyof TagRatioRule,
    value: string | number
  ) => void
  onRemove: (id: string) => void
  onAdd: (groupName: string) => void
  onRemoveGroup: (groupName: string) => void
}

function TagRatioGroupSection({
  groupName,
  items,
  onUpdate,
  onRemove,
  onAdd,
  onRemoveGroup,
}: TagRatioGroupSectionProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(true)

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div className='rounded-lg border'>
        <div className='flex items-center justify-between p-3'>
          <div className='flex items-center gap-2'>
            <CollapsibleTrigger
              render={
                <Button variant='ghost' size='sm' className='h-6 w-6 p-0' />
              }
            >
              {open ? (
                <ChevronUp className='h-4 w-4' />
              ) : (
                <ChevronDown className='h-4 w-4' />
              )}
            </CollapsibleTrigger>
            <span className='font-semibold'>{groupName}</span>
            <StatusBadge variant='info' copyable={false}>
              {items.length} {t('rules')}
            </StatusBadge>
          </div>
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='sm'
              className='h-7 w-7 p-0'
              onClick={() => onAdd(groupName)}
            >
              <Plus className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='sm'
              className='text-destructive h-7 w-7 p-0'
              onClick={() => onRemoveGroup(groupName)}
            >
              <Trash2 className='h-4 w-4' />
            </Button>
          </div>
        </div>
        <CollapsibleContent>
          <div className='space-y-2 border-t p-3'>
            {items.map((rule) => (
              <div
                key={rule._id}
                className='grid gap-2 md:grid-cols-[minmax(0,1fr)_8rem_2rem]'
              >
                <div className='space-y-1'>
                  <Label className='sr-only'>{t('Channel tag')}</Label>
                  <Input
                    value={rule.channelTag}
                    placeholder={t('Channel tag')}
                    onChange={(e) =>
                      onUpdate(rule._id, 'channelTag', e.target.value)
                    }
                  />
                </div>
                <div className='space-y-1'>
                  <Label className='sr-only'>{t('Ratio')}</Label>
                  <Input
                    type='number'
                    min={0}
                    step={0.1}
                    value={rule.ratio}
                    placeholder={t('Ratio')}
                    onChange={(e) =>
                      onUpdate(
                        rule._id,
                        'ratio',
                        normalizeRatio(e.target.value)
                      )
                    }
                  />
                </div>
                <Button
                  variant='ghost'
                  size='sm'
                  className='text-destructive h-8 w-8 p-0'
                  onClick={() => onRemove(rule._id)}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            ))}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  )
}

type ModelTagGroupSectionProps = {
  groupName: string
  items: ModelTagRule[]
  onUpdate: (id: string, field: keyof ModelTagRule, value: string) => void
  onRemove: (id: string) => void
  onAdd: (groupName: string) => void
  onRemoveGroup: (groupName: string) => void
}

function ModelTagGroupSection({
  groupName,
  items,
  onUpdate,
  onRemove,
  onAdd,
  onRemoveGroup,
}: ModelTagGroupSectionProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(true)

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <div className='rounded-lg border'>
        <div className='flex items-center justify-between p-3'>
          <div className='flex items-center gap-2'>
            <CollapsibleTrigger
              render={
                <Button variant='ghost' size='sm' className='h-6 w-6 p-0' />
              }
            >
              {open ? (
                <ChevronUp className='h-4 w-4' />
              ) : (
                <ChevronDown className='h-4 w-4' />
              )}
            </CollapsibleTrigger>
            <span className='font-semibold'>{groupName}</span>
            <StatusBadge variant='info' copyable={false}>
              {items.length} {t('rules')}
            </StatusBadge>
          </div>
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='sm'
              className='h-7 w-7 p-0'
              onClick={() => onAdd(groupName)}
            >
              <Plus className='h-4 w-4' />
            </Button>
            <Button
              variant='ghost'
              size='sm'
              className='text-destructive h-7 w-7 p-0'
              onClick={() => onRemoveGroup(groupName)}
            >
              <Trash2 className='h-4 w-4' />
            </Button>
          </div>
        </div>
        <CollapsibleContent>
          <div className='space-y-2 border-t p-3'>
            {items.map((rule) => (
              <div
                key={rule._id}
                className='grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem]'
              >
                <Input
                  value={rule.modelName}
                  placeholder={t('Model name')}
                  onChange={(e) =>
                    onUpdate(rule._id, 'modelName', e.target.value)
                  }
                />
                <Input
                  value={rule.channelTag}
                  placeholder={t('Forced channel tag')}
                  onChange={(e) =>
                    onUpdate(rule._id, 'channelTag', e.target.value)
                  }
                />
                <Button
                  variant='ghost'
                  size='sm'
                  className='text-destructive h-8 w-8 p-0'
                  onClick={() => onRemove(rule._id)}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            ))}
          </div>
        </CollapsibleContent>
      </div>
    </Collapsible>
  )
}

type PublicGroupBillingRulesEditorProps = {
  tagRatioValue: string
  modelTagValue: string
  groupNames: string[]
  onTagRatioChange: (value: string) => void
  onModelTagChange: (value: string) => void
}

export function PublicGroupBillingRulesEditor({
  tagRatioValue,
  modelTagValue,
  groupNames,
  onTagRatioChange,
  onModelTagChange,
}: PublicGroupBillingRulesEditorProps) {
  const { t } = useTranslation()
  const [tagRatioRules, setTagRatioRules] = useState<TagRatioRule[]>(() =>
    flattenTagRatioRules(tagRatioValue)
  )
  const [modelTagRules, setModelTagRules] = useState<ModelTagRule[]>(() =>
    flattenModelTagRules(modelTagValue)
  )
  const [newTagRatioGroup, setNewTagRatioGroup] = useState('')
  const [newModelTagGroup, setNewModelTagGroup] = useState('')

  const groupedTagRatios = useMemo(
    () => groupRules(tagRatioRules),
    [tagRatioRules]
  )
  const groupedModelTags = useMemo(
    () => groupRules(modelTagRules),
    [modelTagRules]
  )

  const emitTagRatioChange = useCallback(
    (rules: TagRatioRule[]) => {
      setTagRatioRules(rules)
      onTagRatioChange(serializeTagRatioRules(rules))
    },
    [onTagRatioChange]
  )

  const emitModelTagChange = useCallback(
    (rules: ModelTagRule[]) => {
      setModelTagRules(rules)
      onModelTagChange(serializeModelTagRules(rules))
    },
    [onModelTagChange]
  )

  const addTagRatioRule = useCallback(
    (publicGroup: string) => {
      emitTagRatioChange([
        ...tagRatioRules,
        {
          _id: uid('pgtr'),
          publicGroup,
          channelTag: '',
          ratio: 1,
        },
      ])
    },
    [emitTagRatioChange, tagRatioRules]
  )

  const addModelTagRule = useCallback(
    (publicGroup: string) => {
      emitModelTagChange([
        ...modelTagRules,
        {
          _id: uid('pgmt'),
          publicGroup,
          modelName: '',
          channelTag: '',
        },
      ])
    },
    [emitModelTagChange, modelTagRules]
  )

  const addNewTagRatioGroup = useCallback(() => {
    const name = newTagRatioGroup.trim()
    if (!name) return
    addTagRatioRule(name)
    setNewTagRatioGroup('')
  }, [addTagRatioRule, newTagRatioGroup])

  const addNewModelTagGroup = useCallback(() => {
    const name = newModelTagGroup.trim()
    if (!name) return
    addModelTagRule(name)
    setNewModelTagGroup('')
  }, [addModelTagRule, newModelTagGroup])

  const updateTagRatioRule = useCallback(
    (id: string, field: keyof TagRatioRule, value: string | number) => {
      emitTagRatioChange(
        tagRatioRules.map((rule) =>
          rule._id === id ? { ...rule, [field]: value } : rule
        )
      )
    },
    [emitTagRatioChange, tagRatioRules]
  )

  const updateModelTagRule = useCallback(
    (id: string, field: keyof ModelTagRule, value: string) => {
      emitModelTagChange(
        modelTagRules.map((rule) =>
          rule._id === id ? { ...rule, [field]: value } : rule
        )
      )
    },
    [emitModelTagChange, modelTagRules]
  )

  return (
    <div className='space-y-6'>
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Public group tag ratios')}</CardTitle>
          <CardDescription>
            {t(
              'Set effective billing ratios for public groups by channel tag. Matched tags use these ratios first; unmatched requests fall back to the public-group ratio above.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-3'>
            {groupedTagRatios.length === 0 ? (
              <p className='text-muted-foreground py-4 text-center text-sm'>
                {t(
                  'No public-group tag ratio rules yet. Add a public group below to get started.'
                )}
              </p>
            ) : (
              groupedTagRatios.map((group) => (
                <TagRatioGroupSection
                  key={group.name}
                  groupName={group.name}
                  items={group.items}
                  onUpdate={updateTagRatioRule}
                  onRemove={(id) =>
                    emitTagRatioChange(
                      tagRatioRules.filter((rule) => rule._id !== id)
                    )
                  }
                  onAdd={addTagRatioRule}
                  onRemoveGroup={(groupName) =>
                    emitTagRatioChange(
                      tagRatioRules.filter(
                        (rule) => rule.publicGroup !== groupName
                      )
                    )
                  }
                />
              ))
            )}
            <GroupPicker
              datalistId='public-group-tag-ratio-groups'
              value={newTagRatioGroup}
              placeholder={t('Public group name')}
              groupNames={groupNames}
              onChange={setNewTagRatioGroup}
              onAdd={addNewTagRatioGroup}
            />
          </div>
        </CardContent>
      </Card>

      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Public group model tag overrides')}</CardTitle>
          <CardDescription>
            {t(
              'Force a channel tag for specific models inside a public group. These overrides win first; models without overrides still auto-match by channel abilities.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-3'>
            {groupedModelTags.length === 0 ? (
              <p className='text-muted-foreground py-4 text-center text-sm'>
                {t(
                  'No public-group model tag rules yet. Add a public group below to get started.'
                )}
              </p>
            ) : (
              groupedModelTags.map((group) => (
                <ModelTagGroupSection
                  key={group.name}
                  groupName={group.name}
                  items={group.items}
                  onUpdate={updateModelTagRule}
                  onRemove={(id) =>
                    emitModelTagChange(
                      modelTagRules.filter((rule) => rule._id !== id)
                    )
                  }
                  onAdd={addModelTagRule}
                  onRemoveGroup={(groupName) =>
                    emitModelTagChange(
                      modelTagRules.filter(
                        (rule) => rule.publicGroup !== groupName
                      )
                    )
                  }
                />
              ))
            )}
            <GroupPicker
              datalistId='public-group-model-tag-groups'
              value={newModelTagGroup}
              placeholder={t('Public group name')}
              groupNames={groupNames}
              onChange={setNewModelTagGroup}
              onAdd={addNewModelTagGroup}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
