import type { ModelDetails, ModelProfile, ModelProfileDraftInput, ModelProfileInput, ModelProfileResult } from './appTypes';
import {
  applyModelProfile as applyModelProfileAction,
  configuredModelRoleLabel,
  draftModelProfileFromConversation as draftModelProfileFromConversationAction,
  generateRuntimeModelText,
  listModelProfiles,
  modelDetailsLabel,
  modelGeneratedLabel,
  modelProfileDraftLabel,
  modelProfileAppliedLabel,
  modelProfilePreviewLabel,
  modelProfileSavedLabel,
  previewModelProfile as previewModelProfileAction,
  saveModelRoleProvider,
  saveModelProfile as saveModelProfileAction,
  showModelProfile as showModelProfileAction,
  showRuntimeModel
} from './modelActions';

export type ModelControllerContext = {
  getRole: () => string;
  getName: () => string;
  getProvider: () => string;
  getBaseURL: () => string;
  getPrompt: () => string;
  setName: (name: string) => void;
  setDetails: (details: ModelDetails | null) => void;
  setOutput: (output: string) => void;
  getProfileInput: () => ModelProfileInput;
  getProfileDraftInput: () => ModelProfileDraftInput;
  setProfiles: (profiles: ModelProfile[]) => void;
  setProfileResult: (result: ModelProfileResult | null) => void;
  setProfileForm: (profile: ModelProfile) => void;
  setProfileBusy: (busy: boolean) => void;
  setSummary: (summary: string) => void;
  setBusy: (busy: boolean) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  refresh: () => Promise<void>;
  refreshProfiles?: () => Promise<void>;
};

export function createModelController(ctx: ModelControllerContext) {
  async function saveModelRole() {
    ctx.setError('');
    const role = ctx.getRole();
    const name = ctx.getName();
    const provider = ctx.getProvider();
    try {
      await saveModelRoleProvider(role, name, provider, ctx.getBaseURL());
      ctx.pushActivity(configuredModelRoleLabel(role, name, provider));
      await ctx.refresh();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function showModelDetails(name = ctx.getName()) {
    ctx.setError('');
    ctx.setSummary('');
    const selected = name.trim();
    if (!selected) {
      ctx.setError('Choose an installed model first.');
      return;
    }
    try {
      ctx.setBusy(true);
      const details = await showRuntimeModel(selected);
      ctx.setDetails(details);
      ctx.setSummary(modelDetailsLabel(details.name));
      ctx.pushActivity(`model details: ${details.name}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setBusy(false);
    }
  }

  async function selectModel(name: string) {
    ctx.setName(name);
    ctx.setDetails(null);
    ctx.setOutput('');
    ctx.setSummary('');
    await showModelDetails(name);
  }

  async function generateModelText() {
    ctx.setError('');
    ctx.setOutput('');
    ctx.setSummary('');
    const selected = ctx.getName().trim();
    const prompt = ctx.getPrompt().trim();
    if (!selected || !prompt) {
      ctx.setError('Model and prompt are required.');
      return;
    }
    try {
      ctx.setBusy(true);
      const result = await generateRuntimeModelText(selected, prompt);
      ctx.setOutput(result.text);
      ctx.setSummary(modelGeneratedLabel(result.model));
      ctx.pushActivity(`model generate: ${result.model}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setBusy(false);
    }
  }

  async function refreshModelProfiles() {
    const profiles = await listModelProfiles();
    ctx.setProfiles(profiles);
  }

  async function inspectModelProfile(name: string) {
    ctx.setError('');
    const selected = name.trim();
    if (!selected) {
      ctx.setError('Choose a model profile first.');
      return;
    }
    try {
      ctx.setProfileBusy(true);
      const result = await showModelProfileAction(selected);
      ctx.setProfileResult(result);
      ctx.setProfileForm(result.profile);
      ctx.setSummary(`loaded model profile: ${result.profile.name}`);
      ctx.pushActivity(`model profile loaded: ${result.profile.name}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setProfileBusy(false);
    }
  }

  async function previewModelProfile() {
    ctx.setError('');
    const input = ctx.getProfileInput();
    if (!input.name.trim() || !input.baseModel.trim()) {
      ctx.setError('Model profile name and base model are required.');
      return;
    }
    try {
      ctx.setProfileBusy(true);
      const result = await previewModelProfileAction(input);
      ctx.setProfileResult(result);
      ctx.setSummary(modelProfilePreviewLabel(result.profile.name));
      ctx.pushActivity(`model profile preview: ${result.profile.name}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setProfileBusy(false);
    }
  }

  async function draftModelProfileFromConversation() {
    ctx.setError('');
    const input = ctx.getProfileDraftInput();
    if (!input.conversationId.trim()) {
      ctx.setError('Conversation id is required before drafting a model profile.');
      return;
    }
    try {
      ctx.setProfileBusy(true);
      const result = await draftModelProfileFromConversationAction(input);
      ctx.setProfileResult(result);
      ctx.setProfileForm(result.profile);
      ctx.setSummary(modelProfileDraftLabel(result.profile.name));
      ctx.pushActivity(`model profile draft: ${result.profile.name}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setProfileBusy(false);
    }
  }

  async function saveModelProfile() {
    ctx.setError('');
    const input = ctx.getProfileInput();
    if (!input.name.trim() || !input.baseModel.trim()) {
      ctx.setError('Model profile name and base model are required.');
      return;
    }
    try {
      ctx.setProfileBusy(true);
      const result = await saveModelProfileAction(input);
      ctx.setProfileResult(result);
      ctx.setSummary(modelProfileSavedLabel(result.profile.name));
      ctx.pushActivity(`model profile saved: ${result.profile.name}`);
      if (ctx.refreshProfiles) {
        await ctx.refreshProfiles();
      } else {
        await refreshModelProfiles();
      }
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setProfileBusy(false);
    }
  }

  async function applyModelProfile() {
    ctx.setError('');
    const input = ctx.getProfileInput();
    const role = ctx.getRole();
    if (!input.name.trim() || !input.baseModel.trim()) {
      ctx.setError('Save or load a model profile before applying it.');
      return;
    }
    if (!role.trim()) {
      ctx.setError('Choose a model role before applying a profile.');
      return;
    }
    try {
      ctx.setProfileBusy(true);
      const result = await applyModelProfileAction({ name: input.name, role });
      ctx.setProfileResult(result);
      ctx.setProfileForm(result.profile);
      ctx.setSummary(modelProfileAppliedLabel(result.profile.name, result.appliedRole || role));
      ctx.pushActivity(`model profile applied: ${result.profile.name} -> ${result.appliedRole || role}`);
      if (ctx.refreshProfiles) {
        await ctx.refreshProfiles();
      } else {
        await refreshModelProfiles();
      }
      await ctx.refresh();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setProfileBusy(false);
    }
  }

  return {
    saveModelRole,
    selectModel,
    showModelDetails,
    generateModelText,
    refreshModelProfiles,
    inspectModelProfile,
    previewModelProfile,
    draftModelProfileFromConversation,
    saveModelProfile,
    applyModelProfile
  };
}
