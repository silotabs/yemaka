import type { DomainPack, DomainPackActionResult, DomainPackReview, DomainPackSkillStatus, DomainPackTemplate, Skill, SkillActionResult } from './appTypes';
import { call } from './api';
import { domainPackFromResult, domainPackMessage } from './domainPackHelpers';
import { asArray } from './uiHelpers';

export type DomainPackState = {
  packs: DomainPack[];
  templates: DomainPackTemplate[];
  skills: DomainPackSkillStatus[];
};

export type SkillSurface = DomainPackState & {
  catalog: Skill[];
};

export async function listSkillCatalog() {
  return asArray(await call<Skill[]>('ListSkills'));
}

export async function loadDomainPackState(): Promise<DomainPackState> {
  return {
    packs: asArray(await call<DomainPack[]>('ListDomainPacks')),
    templates: asArray(await call<DomainPackTemplate[]>('ListDomainPackTemplates')),
    skills: asArray(await call<DomainPackSkillStatus[]>('ListDomainPackSkills'))
  };
}

export async function loadSkillSurface(): Promise<SkillSurface> {
  const state = await loadDomainPackState();
  return {
    ...state,
    catalog: await listSkillCatalog()
  };
}

export async function installDomainPackSource(source: string) {
  return await call<DomainPack | DomainPackActionResult>('InstallDomainPack', source);
}

export async function installDomainPackTemplateByName(name: string) {
  return await call<DomainPack | DomainPackActionResult>('InstallDomainPackTemplate', name);
}

export async function setDomainPackEnabledState(name: string, enabled: boolean) {
  return await call<DomainPack | DomainPackActionResult>('SetDomainPackEnabled', name, enabled);
}

export async function uninstallDomainPackByName(name: string) {
  return await call<DomainPack | DomainPackActionResult>('UninstallDomainPack', name);
}

export async function reviewDomainPackByName(name: string) {
  return await call<DomainPackReview>('ReviewDomainPack', name);
}

export async function reviewDomainPackTemplateByName(name: string) {
  return await call<DomainPackReview>('ReviewDomainPackTemplate', name);
}

export async function applyCapabilityDomainPack(input: Record<string, unknown>) {
  return await call<DomainPack | DomainPackActionResult>('ApplyCapability', input);
}

export async function validateSkillByName(name: string) {
  return await call<SkillActionResult>('ValidateSkill', name);
}

export async function setSkillEnabledState(name: string, enabled: boolean) {
  return await call<SkillActionResult>('SetSkillEnabled', name, enabled);
}

export async function createSkillFromConversation(conversationId: string) {
  return await call<SkillActionResult>('CreateSkillFromSession', conversationId);
}

export async function improveSkillFromConversation(name: string, conversationId: string) {
  return await call<SkillActionResult>('ImproveSkillFromSession', name, conversationId);
}

export async function importSkillFromPath(path: string) {
  return await call<SkillActionResult>('ImportSkill', path);
}

export async function exportSkillToPath(name: string, path: string) {
  return await call<SkillActionResult>('ExportSkill', name, path);
}

export function domainPackResultSummary(result: DomainPack | DomainPackActionResult, fallback: string) {
  const pack = domainPackFromResult(result);
  return {
    pack,
    message: domainPackMessage(result, fallback),
    action: (result as DomainPackActionResult).action || ''
  };
}

export function skillActionSummary(result: SkillActionResult) {
  return `${result.skill.name}: ${result.message}`;
}

export function skillStillAvailable(skills: Skill[] | null | undefined, name: string) {
  return asArray(skills).some((skill) => skill.name === name);
}
