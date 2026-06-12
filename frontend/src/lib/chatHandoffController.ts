import type { CapabilityHandoff, CapabilityProposal, ChatMessage, Extension, SchedulerHandoff } from './appTypes';
import type { Tab } from './appOptions';
import { createSchedulerJob } from './automationActions';
import {
  capabilityExtensionFormFromHandoff,
  capabilityGenerationInputFromHandoff,
  generatedCapabilityName
} from './capabilityGenerationHelpers';
import { generateCapability, listExtensions } from './extensionActions';
import {
  capabilityHandoffFromEvent,
  capabilityExtensionName as capabilityExtensionNameFor,
  schedulerHandoffDisabled as schedulerHandoffDisabledFor,
  schedulerHandoffFromEvent
} from './handoffHelpers';
import {
  persistCapabilityHandoffForMessage,
  persistSchedulerHandoffForMessage
} from './handoffPersistence';
import {
  schedulerJobCreatedMessage,
  schedulerJobCreatedSummary,
  schedulerJobInputFromHandoff
} from './schedulerJobHelpers';
import { previewJSON } from './uiHelpers';

export type ChatHandoffControllerContext = {
  getMessages: () => ChatMessage[];
  updateAssistantMessageAt: (index: number, updater: (message: ChatMessage) => ChatMessage) => void;
  streamTargetIndex: () => number;
  getExtensions: () => Extension[];
  setExtensions: (items: Extension[]) => void;
  getCapabilityHandoffBusy: () => boolean;
  setCapabilityHandoffBusy: (busy: boolean) => void;
  getSchedulerHandoffBusy: () => boolean;
  setSchedulerHandoffBusy: (busy: boolean) => void;
  getExtensionGenerateName: () => string;
  setExtensionGenerateName: (value: string) => void;
  getExtensionGenerateDescription: () => string;
  setExtensionGenerateDescription: (value: string) => void;
  getCapabilityNextSteps: () => string[];
  setCapabilityNextSteps: (steps: string[]) => void;
  setCapabilityProposal: (proposal: CapabilityProposal | null) => void;
  setCapabilityProposalRequest: (request: string) => void;
  setExtensionProposalRequest: (request: string) => void;
  setExtensionSummary: (summary: string) => void;
  setExtensionRunName: (name: string) => void;
  setExtensionRunOutput: (output: string) => void;
  getActiveConversationId: () => string;
  setActiveDomainPackHandoff: (handoff: CapabilityHandoff | null) => void;
  getDomainPackInstallPath: () => string;
  setDomainPackInstallPath: (path: string) => void;
  setDomainPackSummary: (summary: string) => void;
  domainPackTemplateInstallPath: (handoff: CapabilityHandoff | null | undefined) => string;
  refreshSkillsView: () => Promise<void>;
  reviewExtension: (name: string) => Promise<void>;
  refreshExtensions: () => Promise<void>;
  refreshAutomation: () => Promise<void>;
  setJobSummary: (summary: string) => void;
  setActiveTab: (tab: Tab) => void;
  afterOpen: () => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
};

export function createChatHandoffController(ctx: ChatHandoffControllerContext) {
  function updateCapabilityHandoff(index: number, updater: (handoff: CapabilityHandoff) => CapabilityHandoff) {
    ctx.updateAssistantMessageAt(index, (message) => {
      if (!message.capabilityHandoff) return message;
      const capabilityHandoff = updater(message.capabilityHandoff);
      const next = { ...message, capabilityHandoff };
      persistCapabilityHandoffForMessage(next, capabilityHandoff);
      return next;
    });
  }

  function rememberCapabilityHandoff(data: Record<string, string> | undefined) {
    const handoff = capabilityHandoffFromEvent(data);
    if (!handoff) return;
    const target = ctx.streamTargetIndex();
    ctx.updateAssistantMessageAt(target, (message) => {
      const capabilityHandoff: CapabilityHandoff = {
        ...handoff,
        status: message.capabilityHandoff?.status ?? handoff.status,
        message: message.capabilityHandoff?.message
      };
      const next = { ...message, capabilityHandoff };
      persistCapabilityHandoffForMessage(next, capabilityHandoff);
      return next;
    });
  }

  function updateSchedulerHandoff(index: number, updater: (handoff: SchedulerHandoff) => SchedulerHandoff) {
    ctx.updateAssistantMessageAt(index, (message) => {
      if (!message.schedulerHandoff) return message;
      const schedulerHandoff = updater(message.schedulerHandoff);
      const next = { ...message, schedulerHandoff };
      persistSchedulerHandoffForMessage(next, schedulerHandoff);
      return next;
    });
  }

  function rememberSchedulerHandoff(data: Record<string, string> | undefined) {
    const handoff = schedulerHandoffFromEvent(data);
    if (!handoff) return;
    const target = ctx.streamTargetIndex();
    ctx.updateAssistantMessageAt(target, (message) => {
      const schedulerHandoff: SchedulerHandoff = {
        ...handoff,
        status: message.schedulerHandoff?.status ?? handoff.status,
        message: message.schedulerHandoff?.message,
        job: message.schedulerHandoff?.job
      };
      const next = { ...message, schedulerHandoff };
      persistSchedulerHandoffForMessage(next, schedulerHandoff);
      return next;
    });
  }

  function schedulerHandoffDisabled(handoff: SchedulerHandoff) {
    return schedulerHandoffDisabledFor(handoff, ctx.getSchedulerHandoffBusy());
  }

  function setSchedulerHandoffInput(index: number, value: string) {
    updateSchedulerHandoff(index, (handoff) => ({ ...handoff, inputJSON: value }));
  }

  function setCapabilityRunAfterGenerate(index: number, enabled: boolean) {
    updateCapabilityHandoff(index, (handoff) => ({
      ...handoff,
      runAfterGenerate: enabled,
      runInputJSON: handoff.runInputJSON || '{}'
    }));
  }

  function setCapabilityRunInput(index: number, value: string) {
    updateCapabilityHandoff(index, (handoff) => ({ ...handoff, runInputJSON: value }));
  }

  function setCapabilityScheduleAfterGenerate(index: number, enabled: boolean) {
    updateCapabilityHandoff(index, (handoff) => ({
      ...handoff,
      scheduleAfterGenerate: enabled,
      scheduleType: handoff.scheduleType || 'manual',
      scheduleExpr: handoff.scheduleExpr || '',
      scheduleEnabled: handoff.scheduleEnabled ?? false,
      scheduleInputJSON: handoff.scheduleInputJSON || '{}'
    }));
  }

  function setCapabilityScheduleType(index: number, value: string) {
    updateCapabilityHandoff(index, (handoff) => ({ ...handoff, scheduleType: value }));
  }

  function setCapabilityScheduleExpr(index: number, value: string) {
    updateCapabilityHandoff(index, (handoff) => ({ ...handoff, scheduleExpr: value }));
  }

  function setCapabilityScheduleEnabled(index: number, enabled: boolean) {
    updateCapabilityHandoff(index, (handoff) => ({ ...handoff, scheduleEnabled: enabled }));
  }

  function setCapabilityScheduleInput(index: number, value: string) {
    updateCapabilityHandoff(index, (handoff) => ({ ...handoff, scheduleInputJSON: value }));
  }

  function capabilityExtensionName(handoff: CapabilityHandoff | undefined) {
    return capabilityExtensionNameFor(handoff);
  }

  function openCapabilityInExtensions(handoff: CapabilityHandoff) {
    ctx.setActiveTab('extensions');
    const form = capabilityExtensionFormFromHandoff(
      handoff,
      ctx.getExtensionGenerateName(),
      ctx.getExtensionGenerateDescription(),
      ctx.getCapabilityNextSteps()
    );
    ctx.setExtensionProposalRequest(form.request);
    ctx.setExtensionGenerateName(form.name);
    ctx.setExtensionGenerateDescription(form.description);
    ctx.setCapabilityProposal(form.proposal);
    ctx.setCapabilityProposalRequest(form.proposalRequest);
    ctx.setCapabilityNextSteps(form.nextSteps);
    ctx.setExtensionSummary(form.summary);
    if (handoff.generatedName) {
      void ctx.reviewExtension(handoff.generatedName);
    }
    ctx.afterOpen();
  }

  function openMessageCapabilityInExtensions(index: number) {
    const handoff = ctx.getMessages()[index]?.capabilityHandoff;
    if (!handoff) return;
    openCapabilityInExtensions(handoff);
  }

  function openDomainPacksFromHandoff(handoff: CapabilityHandoff) {
    ctx.setActiveTab('skills');
    ctx.setActiveDomainPackHandoff(handoff);
    const installPath = ctx.domainPackTemplateInstallPath(handoff);
    if (installPath && !ctx.getDomainPackInstallPath().trim()) {
      ctx.setDomainPackInstallPath(installPath);
    }
    ctx.setDomainPackSummary(handoff.configureHint || handoff.suggestedAction || handoff.capabilitySummary || '');
    void ctx.refreshSkillsView();
    ctx.afterOpen();
  }

  function openMessageDomainPacks(index: number) {
    const handoff = ctx.getMessages()[index]?.capabilityHandoff;
    if (!handoff) return;
    openDomainPacksFromHandoff(handoff);
  }

  async function approveChatCapability(index: number) {
    const handoff = ctx.getMessages()[index]?.capabilityHandoff;
    if (!handoff || ctx.getCapabilityHandoffBusy() || !handoff.canGenerate || handoff.status === 'generated') return;
    ctx.setCapabilityHandoffBusy(true);
    ctx.setError('');
    updateCapabilityHandoff(index, (current) => ({
      ...current,
      status: 'generating',
      message: 'Generating, testing, and registering the smallest profile-local extension...'
    }));
    try {
      const targetName = capabilityExtensionName(handoff);
      if (targetName) {
        const latest = await listExtensions();
        ctx.setExtensions(latest);
        const existing = latest.find((extension) => extension.name === targetName);
        if (existing) {
          updateCapabilityHandoff(index, (current) => ({
            ...current,
            status: 'generated',
            generatedName: existing.name,
            message: `${existing.name} already exists. Review and reuse it instead of generating a duplicate.`
          }));
          ctx.setExtensionRunName(existing.name);
          await ctx.reviewExtension(existing.name);
          ctx.pushActivity(`capability reuse: ${existing.name}`);
          return;
        }
      }
      const result = await generateCapability(capabilityGenerationInputFromHandoff(handoff, ctx.getActiveConversationId()));
      const generatedName = generatedCapabilityName(result, handoff.name, handoff.title);
      updateCapabilityHandoff(index, (current) => ({
        ...current,
        status: 'generated',
        generatedName,
        message: result.message || `Generated ${generatedName}`,
        nextSteps: result.nextSteps ?? [],
        run: result.run,
        schedule: result.schedule
      }));
      ctx.setExtensionRunName(generatedName);
      ctx.pushActivity(`capability generated: ${generatedName}`);
      if (result.run) {
        ctx.setExtensionRunOutput(previewJSON(result.run));
        ctx.pushActivity(`capability run: ${result.run.extension} ${result.run.status}`);
      }
      if (result.schedule) {
        ctx.setJobSummary(`created ${result.schedule.id}`);
        ctx.pushActivity(`capability job: ${result.schedule.name || result.schedule.id}`);
        await ctx.refreshAutomation();
      }
      await ctx.refreshExtensions();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      updateCapabilityHandoff(index, (current) => ({
        ...current,
        status: 'failed',
        message
      }));
    } finally {
      ctx.setCapabilityHandoffBusy(false);
    }
  }

  async function createSchedulerJobFromChat(index: number) {
    const handoff = ctx.getMessages()[index]?.schedulerHandoff;
    if (!handoff || schedulerHandoffDisabled(handoff)) return;
    ctx.setSchedulerHandoffBusy(true);
    ctx.setError('');
    ctx.setJobSummary('');
    updateSchedulerHandoff(index, (current) => ({
      ...current,
      status: 'creating',
      message: 'Creating an approved scheduler job record. It will stay disabled.'
    }));
    try {
      const job = await createSchedulerJob(schedulerJobInputFromHandoff(handoff, ctx.getActiveConversationId()));
      updateSchedulerHandoff(index, (current) => ({
        ...current,
        status: 'created',
        job,
        message: schedulerJobCreatedMessage(job)
      }));
      ctx.setJobSummary(schedulerJobCreatedSummary(job));
      ctx.pushActivity(`job created from chat: ${job.name || job.id}`);
      await ctx.refreshAutomation();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setError(message);
      updateSchedulerHandoff(index, (current) => ({
        ...current,
        status: 'failed',
        message
      }));
    } finally {
      ctx.setSchedulerHandoffBusy(false);
    }
  }

  return {
    updateCapabilityHandoff,
    rememberCapabilityHandoff,
    updateSchedulerHandoff,
    rememberSchedulerHandoff,
    schedulerHandoffDisabled,
    setSchedulerHandoffInput,
    setCapabilityRunAfterGenerate,
    setCapabilityRunInput,
    setCapabilityScheduleAfterGenerate,
    setCapabilityScheduleType,
    setCapabilityScheduleExpr,
    setCapabilityScheduleEnabled,
    setCapabilityScheduleInput,
    capabilityExtensionName,
    openCapabilityInExtensions,
    openMessageCapabilityInExtensions,
    openDomainPacksFromHandoff,
    openMessageDomainPacks,
    approveChatCapability,
    createSchedulerJobFromChat
  };
}
