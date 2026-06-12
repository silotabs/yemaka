import type { CapabilityHandoff, DomainPack, DomainPackReview, DomainPackSkillStatus, DomainPackTemplate, Skill } from './appTypes';
import {
  domainPackForHandoff as domainPackForHandoffFor,
  domainPackHandoffApplyInput as domainPackHandoffApplyInputFor,
  domainPackHandoffBlockedSummary,
  domainPackHandoffName as domainPackHandoffNameFor,
  domainPackHandoffSummary,
  domainPackInstallSummary,
  domainPackTemplateInstallPath as domainPackTemplateInstallPathFor,
  domainPackTemplateInstallSummary,
  domainPackUninstallConfirmMessage,
  domainPackUninstallSummary,
  domainPackUpdateSummary,
  isRemotePackPath
} from './domainPackHelpers';
import {
  applyCapabilityDomainPack,
  createSkillFromConversation,
  domainPackResultSummary,
  exportSkillToPath,
  improveSkillFromConversation,
  importSkillFromPath,
  installDomainPackSource,
  installDomainPackTemplateByName,
  loadDomainPackState as loadDomainPackStateAction,
  loadSkillSurface,
  reviewDomainPackByName,
  reviewDomainPackTemplateByName,
  setDomainPackEnabledState,
  setSkillEnabledState,
  skillActionSummary,
  skillStillAvailable,
  uninstallDomainPackByName,
  validateSkillByName
} from './skillActions';

export type SkillsControllerContext = {
  getSkills: () => Skill[];
  setSkills: (skills: Skill[]) => void;
  getDomainPacks: () => DomainPack[];
  setDomainPacks: (packs: DomainPack[]) => void;
  setDomainPackTemplates: (templates: DomainPackTemplate[]) => void;
  getDomainPackTemplates: () => DomainPackTemplate[];
  setDomainPackSkills: (skills: DomainPackSkillStatus[]) => void;
  getDomainPackSkills: () => DomainPackSkillStatus[];
  getDomainPackInstallPath: () => string;
  setDomainPackInstallPath: (path: string) => void;
  getDomainPackBusy: () => boolean;
  setDomainPackBusy: (busy: boolean) => void;
  setDomainPackSummary: (summary: string) => void;
  setDomainPackReview: (review: DomainPackReview | null) => void;
  getSelectedSkill: () => string;
  setSelectedSkill: (skill: string) => void;
  getSkillSessionId: () => string;
  setSkillSessionId: (id: string) => void;
  getSkillImproveName: () => string;
  setSkillImproveName: (name: string) => void;
  getSkillImportPath: () => string;
  setSkillImportPath: (path: string) => void;
  getSkillExportPath: () => string;
  setSkillSummary: (summary: string) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  confirm: (message: string) => Promise<boolean> | boolean;
};

export function createSkillsController(ctx: SkillsControllerContext) {
  function replaceDomainPack(pack: DomainPack) {
    const name = String(pack?.name || '').trim();
    if (!name) return;
    const current = ctx.getDomainPacks();
    const found = current.some((item) => item.name === name);
    ctx.setDomainPacks(found ? current.map((item) => (item.name === name ? pack : item)) : [pack, ...current]);
  }

  function removeDomainPack(name: string) {
    ctx.setDomainPacks(ctx.getDomainPacks().filter((pack) => pack.name !== name));
  }

  function replaceSkill(skill: Skill) {
    const name = String(skill?.name || '').trim();
    if (!name) return;
    const current = ctx.getSkills();
    const found = current.some((item) => item.name === name);
    ctx.setSkills(found ? current.map((item) => (item.name === name ? skill : item)) : [skill, ...current]);
  }

  function applySkillSurface(surface: Awaited<ReturnType<typeof loadSkillSurface>>) {
    ctx.setDomainPacks(surface.packs);
    ctx.setDomainPackTemplates(surface.templates);
    ctx.setDomainPackSkills(surface.skills);
    ctx.setSkills(surface.catalog);
  }

  function domainPackHandoffName(handoff: CapabilityHandoff | undefined) {
    return domainPackHandoffNameFor(handoff);
  }

  function domainPackForHandoff(handoff: CapabilityHandoff | undefined) {
    return domainPackForHandoffFor(ctx.getDomainPacks(), handoff);
  }

  function domainPackTemplateInstallPath(handoff: CapabilityHandoff | null | undefined) {
    return domainPackTemplateInstallPathFor(handoff, domainPackHandoffName(handoff ?? undefined));
  }

  function domainPackHandoffApplyInput(handoff: CapabilityHandoff, installed: DomainPack | null) {
    return domainPackHandoffApplyInputFor(handoff, installed, domainPackHandoffName(handoff), domainPackTemplateInstallPath(handoff));
  }

  async function loadDomainPackState() {
    const state = await loadDomainPackStateAction();
    ctx.setDomainPacks(state.packs);
    ctx.setDomainPackTemplates(state.templates);
    ctx.setDomainPackSkills(state.skills);
  }

  async function refreshSkillsView(silent = false) {
    if (!silent) {
      ctx.setError('');
      ctx.setDomainPackBusy(true);
    }
    try {
      const surface = await loadSkillSurface();
      applySkillSurface(surface);
      if (!silent) ctx.pushActivity(`skills: ${surface.catalog.length}`);
    } catch (err) {
      if (!silent) {
        const message = err instanceof Error ? err.message : String(err);
        ctx.setError(message);
        ctx.setDomainPackSummary(`Refresh failed: ${message}`);
      }
    } finally {
      if (!silent) ctx.setDomainPackBusy(false);
    }
  }

  async function installDomainPack() {
    const source = ctx.getDomainPackInstallPath().trim();
    if (!source) return;
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackReview(null);
    if (isRemotePackPath(source)) {
      ctx.setDomainPackSummary('Install a local pack folder. Remote pack installs are not supported.');
      return;
    }
    ctx.setDomainPackBusy(true);
    try {
      const result = await installDomainPackSource(source);
      const { pack, message } = domainPackResultSummary(result, 'installed');
      replaceDomainPack(pack);
      ctx.setDomainPackSummary(domainPackInstallSummary(pack.name, message));
      ctx.setDomainPackInstallPath('');
      ctx.pushActivity(`domain pack installed: ${pack.name}`);
      await refreshSkillsView(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Install failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function installDomainPackTemplate(name: string) {
    const templateName = name.trim();
    if (!templateName) return;
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackReview(null);
    ctx.setDomainPackBusy(true);
    try {
      const result = await installDomainPackTemplateByName(templateName);
      const { pack, message } = domainPackResultSummary(result, 'installed');
      replaceDomainPack(pack);
      ctx.setDomainPackSummary(domainPackTemplateInstallSummary(pack.name, message));
      ctx.pushActivity(`domain pack template installed: ${pack.name}`);
      await refreshSkillsView(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Template install failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function setDomainPackEnabled(name: string, enabled: boolean) {
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackReview(null);
    ctx.setDomainPackBusy(true);
    try {
      const result = await setDomainPackEnabledState(name, enabled);
      const { pack, message } = domainPackResultSummary(result, enabled ? 'enabled' : 'disabled');
      replaceDomainPack(pack);
      ctx.setDomainPackSummary(domainPackUpdateSummary(pack.name, message));
      ctx.pushActivity(`domain pack ${enabled ? 'enabled' : 'disabled'}: ${pack.name}`);
      await refreshSkillsView(true);
      const selected = ctx.getSelectedSkill();
      if (!enabled && selected && !skillStillAvailable(ctx.getSkills(), selected)) {
        ctx.setSelectedSkill('');
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Update failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function uninstallDomainPack(pack: DomainPack) {
    const name = String(pack?.name || '').trim();
    if (!name || ctx.getDomainPackBusy()) return;
    if (!(await ctx.confirm(domainPackUninstallConfirmMessage(name)))) return;
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackReview(null);
    ctx.setDomainPackBusy(true);
    try {
      const result = await uninstallDomainPackByName(name);
      const { pack: removed, message } = domainPackResultSummary(result, 'uninstalled');
      removeDomainPack(name);
      ctx.setDomainPackSummary(domainPackUninstallSummary(removed.name, name, message));
      ctx.pushActivity(`domain pack uninstalled: ${removed.name || name}`);
      await refreshSkillsView(true);
      const selected = ctx.getSelectedSkill();
      if (selected && !skillStillAvailable(ctx.getSkills(), selected)) {
        ctx.setSelectedSkill('');
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Uninstall failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function reviewDomainPack(pack: DomainPack) {
    const name = String(pack?.name || '').trim();
    if (!name || ctx.getDomainPackBusy()) return;
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackBusy(true);
    try {
      const review = await reviewDomainPackByName(name);
      ctx.setDomainPackReview(review);
      ctx.setDomainPackSummary(`${review.name}: review loaded`);
      ctx.pushActivity(`domain pack reviewed: ${review.name}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Review failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function reviewDomainPackTemplate(name: string) {
    const templateName = String(name || '').trim();
    if (!templateName || ctx.getDomainPackBusy()) return;
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackBusy(true);
    try {
      const review = await reviewDomainPackTemplateByName(templateName);
      ctx.setDomainPackReview(review);
      ctx.setDomainPackSummary(`${review.name}: template review loaded`);
      ctx.pushActivity(`domain pack template reviewed: ${review.name}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Template review failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function applyDomainPackHandoff(handoff: CapabilityHandoff | null | undefined) {
    if (!handoff || ctx.getDomainPackBusy()) return;
    ctx.setError('');
    ctx.setDomainPackSummary('');
    ctx.setDomainPackReview(null);
    const name = domainPackHandoffName(handoff);
    const installed = domainPackForHandoff(handoff);
    const installPath = domainPackTemplateInstallPath(handoff);
    const blockedSummary = domainPackHandoffBlockedSummary(handoff, installed, name, installPath);
    if (blockedSummary) {
      ctx.setDomainPackSummary(blockedSummary);
      return;
    }

    ctx.setDomainPackBusy(true);
    try {
      const result = await applyCapabilityDomainPack(domainPackHandoffApplyInput(handoff, installed));
      const summary = domainPackResultSummary(result, installed ? 'enabled' : 'installed');
      const pack = summary.pack;
      const action = summary.action || (installed ? 'enabled' : 'installed');
      replaceDomainPack(pack);
      ctx.setDomainPackSummary(domainPackHandoffSummary(pack.name, name, action, summary.message));
      ctx.setDomainPackInstallPath('');
      ctx.pushActivity(`domain pack handoff ${action}: ${pack.name || name}`);
      await refreshSkillsView(true);
      const selected = ctx.getSelectedSkill();
      if (action === 'enabled' && selected && !skillStillAvailable(ctx.getSkills(), selected)) {
        ctx.setSelectedSkill('');
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      ctx.setDomainPackSummary(`Handoff apply failed: ${message}`);
    } finally {
      ctx.setDomainPackBusy(false);
    }
  }

  async function validateSkill(name: string) {
    ctx.setError('');
    try {
      const result = await validateSkillByName(name);
      ctx.setSkillSummary(skillActionSummary(result));
      ctx.pushActivity(`skill valid: ${result.skill.name}`);
      replaceSkill(result.skill);
      await refreshSkillsView(true);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function setSkillEnabled(name: string, enabled: boolean) {
    ctx.setError('');
    try {
      const result = await setSkillEnabledState(name, enabled);
      ctx.setSkillSummary(skillActionSummary(result));
      ctx.pushActivity(`skill ${result.message}: ${result.skill.name}`);
      replaceSkill(result.skill);
      await refreshSkillsView(true);
      if (!enabled && ctx.getSelectedSkill() === name) {
        ctx.setSelectedSkill('');
      }
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function createSkillFromSession() {
    ctx.setError('');
    try {
      const result = await createSkillFromConversation(ctx.getSkillSessionId());
      ctx.setSkillSummary(`created ${result.skill.name}`);
      ctx.setSkillSessionId('');
      ctx.pushActivity(`skill created: ${result.skill.name}`);
      replaceSkill(result.skill);
      await refreshSkillsView(true);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function improveSkillFromSession() {
    ctx.setError('');
    try {
      const result = await improveSkillFromConversation(ctx.getSkillImproveName(), ctx.getSkillSessionId());
      ctx.setSkillSummary(`improved ${result.skill.name}`);
      ctx.setSkillImproveName('');
      ctx.setSkillSessionId('');
      ctx.pushActivity(`skill improved: ${result.skill.name}`);
      replaceSkill(result.skill);
      await refreshSkillsView(true);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function importSkill() {
    ctx.setError('');
    try {
      const result = await importSkillFromPath(ctx.getSkillImportPath());
      ctx.setSkillSummary(`imported ${result.skill.name}`);
      ctx.setSkillImportPath('');
      ctx.pushActivity(`skill imported: ${result.skill.name}`);
      replaceSkill(result.skill);
      await refreshSkillsView(true);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function exportSkill(name: string) {
    ctx.setError('');
    try {
      const result = await exportSkillToPath(name, ctx.getSkillExportPath());
      ctx.setSkillSummary(`exported ${result.skill.name} to ${result.path}`);
      ctx.pushActivity(`skill exported: ${result.skill.name}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    loadDomainPackState,
    refreshSkillsView,
    installDomainPack,
    installDomainPackTemplate,
    reviewDomainPack,
    reviewDomainPackTemplate,
    setDomainPackEnabled,
    uninstallDomainPack,
    applyDomainPackHandoff,
    validateSkill,
    setSkillEnabled,
    createSkillFromSession,
    improveSkillFromSession,
    importSkill,
    exportSkill,
    domainPackHandoffName,
    domainPackForHandoff,
    domainPackTemplateInstallPath
  };
}
