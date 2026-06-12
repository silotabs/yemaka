export type DomainPackLike = {
  name?: string;
  enabled?: boolean;
  valid?: boolean;
};

export type DomainPackActionLike<TPack> = {
  pack?: TPack;
  message?: string;
};

export type DomainPackTemplateLike = {
  name?: string;
  installed?: boolean;
  safetySummary?: string;
  defaultState?: string;
};

export type DomainPackSkillLike = {
  packName?: string;
};

export type CapabilityHandoffLike = {
  kind?: string;
  name?: string;
  existingCapability?: string;
  capabilityAction?: string;
  capabilityState?: string;
  packSource?: string;
  packDir?: string;
  installCommand?: string;
};

export function domainPackFromResult<TPack>(result: TPack | DomainPackActionLike<TPack>): TPack {
  return (result as DomainPackActionLike<TPack>).pack ?? (result as TPack);
}

export function domainPackMessage<TPack>(result: TPack | DomainPackActionLike<TPack>, fallback: string) {
  return (result as DomainPackActionLike<TPack>).message || fallback;
}

export function domainPackTemplateSafetyText(template: DomainPackTemplateLike) {
  return template.safetySummary || `Installs ${template.defaultState || 'disabled'}; enable separately when ready.`;
}

export function domainPackSkillRefs<TSkill extends DomainPackSkillLike>(skills: TSkill[] | null | undefined, packName: string) {
  return (skills ?? []).filter((skill) => skill.packName === packName);
}

export function availableDomainPackTemplates<TPack extends DomainPackLike, TTemplate extends DomainPackTemplateLike>(
  packs: TPack[] | null | undefined,
  templates: TTemplate[] | null | undefined
) {
  const installed = new Set((packs ?? []).map((pack) => pack.name));
  return (templates ?? []).filter((template) => !template.installed && !installed.has(template.name));
}

export function isRemotePackPath(path: string) {
  return /^(https?|ssh|git):\/\//i.test(path.trim());
}

export function domainPackTemplateInstallPath(handoff: CapabilityHandoffLike | null | undefined, handoffName: string) {
  const explicit = String(handoff?.packDir || '').trim();
  if (explicit && !isRemotePackPath(explicit)) return explicit;
  if (handoff?.packSource === 'domain_pack_template' && handoffName) return `packs/templates/${handoffName}`;
  return '';
}

export function isDomainPackHandoff(handoff: CapabilityHandoffLike | undefined) {
  return String(handoff?.kind || '').trim() === 'domain_pack';
}

export function domainPackHandoffName(handoff: CapabilityHandoffLike | undefined) {
  if (!handoff) return '';
  return String(handoff.existingCapability || handoff.name || '').trim();
}

export function domainPackForHandoff<TPack extends DomainPackLike>(packs: TPack[] | null | undefined, handoff: CapabilityHandoffLike | undefined) {
  const name = domainPackHandoffName(handoff);
  if (!name) return null;
  return (packs ?? []).find((pack) => pack.name === name) ?? null;
}

export function domainPackSetupLabel(handoff: CapabilityHandoffLike, pack: DomainPackLike | null | undefined) {
  if (pack?.enabled) return 'Domain pack enabled';
  if (pack && !pack.valid) return 'Repair in Domain Packs';
  if (pack) return 'Enable in Domain Packs';
  if (handoff.capabilityState === 'needs_config' || handoff.capabilityAction === 'ask_config') return 'Setup in Domain Packs';
  if (handoff.packSource === 'domain_pack_template') return 'Install pack template';
  return 'Open Domain Packs';
}

export function domainPackHandoffActionLabel(handoff: CapabilityHandoffLike | null | undefined, pack: DomainPackLike | null | undefined, installPath: string) {
  if (!handoff) return '';
  if (pack?.enabled) return 'Already enabled';
  if (pack && pack.valid) return 'Enable domain pack';
  if (pack && !pack.valid) return 'Cannot enable invalid pack';
  if (installPath) return 'Install pack template';
  return 'No UI action available';
}

export function domainPackHandoffActionDisabled(
  handoff: CapabilityHandoffLike | null | undefined,
  busy: boolean,
  pack: DomainPackLike | null | undefined,
  installPath: string,
  handoffName: string
) {
  if (!handoff || busy) return true;
  if (pack) return Boolean(pack.enabled || !pack.valid);
  return handoff.packSource !== 'domain_pack_template' || (!handoffName && !installPath);
}

export function domainPackHandoffApplyInput(handoff: CapabilityHandoffLike, installed: DomainPackLike | null, handoffName: string, installPath: string) {
  return {
    approved: true,
    kind: 'domain_pack',
    name: handoff.name || handoffName,
    existingCapability: handoff.existingCapability || '',
    packSource: handoff.packSource || (installed ? 'domain_pack' : 'domain_pack_template'),
    packDir: handoff.packDir || installPath
  };
}

export function domainPackInstallSummary(packName: string, message: string) {
  return `${packName}: ${message}. Packs stay disabled until enabled here.`;
}

export function domainPackTemplateInstallSummary(packName: string, message: string) {
  return `${packName}: ${message}. Enable it explicitly when ready.`;
}

export function domainPackUpdateSummary(packName: string, message: string) {
  return `${packName}: ${message}`;
}

export function domainPackUninstallConfirmMessage(name: string) {
  return `Uninstall ${name} from this profile? Its pack skills will stop loading, but the original template or source folder is not changed.`;
}

export function domainPackUninstallSummary(packName: string, fallbackName: string, message: string) {
  return `${packName || fallbackName}: ${message}`;
}

export function domainPackHandoffBlockedSummary(
  handoff: CapabilityHandoffLike,
  installed: DomainPackLike | null | undefined,
  name: string,
  installPath: string
) {
  if (installed?.enabled) return `${installed.name}: already enabled.`;
  if (installed && !installed.valid) return `${installed.name}: cannot enable until validation is repaired.`;
  if (installed) return '';
  if (handoff.packSource !== 'domain_pack_template' || (!name && !installPath)) {
    return handoff.installCommand ? `Use CLI for this handoff: ${handoff.installCommand}` : 'This handoff does not include a pack the UI can apply.';
  }
  return '';
}

export function domainPackHandoffSummary(packName: string, fallbackName: string, action: string, message: string) {
  const label = packName || fallbackName;
  return action === 'enabled' ? `${label}: ${message}` : `${label}: ${message}. Enable it explicitly when ready.`;
}
