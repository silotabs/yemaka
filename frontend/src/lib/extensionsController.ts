import type { CapabilityProposal, Extension, ExtensionFailureTrend, ExtensionReview } from './appTypes';
import {
  capabilityGenerationInputFromForm,
  generatedCapabilityName,
  generatedExtensionNameFromLegacyResult
} from './capabilityGenerationHelpers';
import {
  deleteExtensionByName,
  extensionActionSummary,
  generateCapability,
  generateExtensionFromDescription,
  listExtensionFailures,
  listExtensions,
  loadExtensionSurface,
  proposeCapability,
  registerExtensionByName,
  reviewExtensionByName,
  rollbackExtensionTarget,
  runExtensionByName,
  setExtensionEnabledState,
  testExtensionByName,
  validateExtensionByName
} from './extensionActions';
import {
  extensionRegisterSummary,
  extensionReuseSummary,
  extensionReviewFormState,
  extensionRunInputFromJSON,
  extensionRunSummary,
  extensionTestSummary,
  reviewedExtensionRunnerState
} from './extensionPageHelpers';
import { previewJSON } from './uiHelpers';

export type ExtensionsControllerContext = {
  getExtensionProposalRequest: () => string;
  setExtensionProposalRequest: (value: string) => void;
  getExtensionGenerateName: () => string;
  setExtensionGenerateName: (value: string) => void;
  getExtensionGenerateDescription: () => string;
  setExtensionGenerateDescription: (value: string) => void;
  getExtensionGenerateRun: () => boolean;
  getExtensionGenerateJob: () => boolean;
  getExtensionJobScheduleType: () => string;
  getExtensionJobScheduleExpr: () => string;
  getExtensionJobEnabled: () => boolean;
  getExtensionJobInputJSON: () => string;
  getExtensionRunName: () => string;
  setExtensionRunName: (value: string) => void;
  getExtensionRunInputJSON: () => string;
  setExtensionRunInputJSON: (value: string) => void;
  setExtensionRunOutput: (value: string) => void;
  getExtensionRollbackTarget: () => string;
  setExtensionRollbackTarget: (value: string) => void;
  getExtensionReview: () => ExtensionReview | null;
  setExtensionReview: (review: ExtensionReview | null) => void;
  setExtensionReviewOpenFile: (path: string) => void;
  setExtensionReviewBusy: (busy: boolean) => void;
  setExtensions: (extensions: Extension[]) => void;
  setExtensionFailures: (failures: ExtensionFailureTrend[]) => void;
  setExtensionBusy: (busy: boolean) => void;
  setExtensionSummary: (summary: string) => void;
  getCapabilityProposal: () => CapabilityProposal | null;
  setCapabilityProposal: (proposal: CapabilityProposal | null) => void;
  getCapabilityProposalRequest: () => string;
  setCapabilityProposalRequest: (request: string) => void;
  setCapabilityNextSteps: (steps: string[]) => void;
  setJobSummary: (summary: string) => void;
  setError: (message: string) => void;
  setActiveTab: (tab: 'extensions') => void;
  refreshAutomation: () => Promise<void>;
  pushActivity: (line: string) => void;
};

export function createExtensionsController(ctx: ExtensionsControllerContext) {
  async function refreshExtensions(silent = false) {
    if (!silent) {
      ctx.setError('');
      ctx.setExtensionBusy(true);
    }
    try {
      const extensionSurface = await loadExtensionSurface(20);
      ctx.setExtensions(extensionSurface.extensions);
      ctx.setExtensionFailures(extensionSurface.failures);
      if (!silent) ctx.pushActivity(`extensions: ${extensionSurface.extensions.length}`);
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (!silent) ctx.setExtensionBusy(false);
    }
  }

  async function reviewExtension(name: string) {
    const trimmed = String(name || '').trim();
    if (!trimmed) return;
    ctx.setError('');
    ctx.setExtensionReviewBusy(true);
    try {
      const review = await reviewExtensionByName(trimmed);
      const form = extensionReviewFormState(review, trimmed, ctx.getExtensionRunInputJSON());
      ctx.setExtensionReview(review);
      ctx.setExtensionReviewOpenFile(form.reviewOpenFile);
      ctx.setExtensionRunName(form.runName);
      ctx.setExtensionRunInputJSON(form.runInputJSON);
      ctx.setExtensionSummary(form.summary);
      ctx.pushActivity(form.activity);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setExtensionReviewBusy(false);
    }
  }

  function useReviewedExtensionInRunner() {
    const review = ctx.getExtensionReview();
    if (!review) return;
    const runner = reviewedExtensionRunnerState(review, ctx.getExtensionRunName(), ctx.getExtensionRunInputJSON());
    ctx.setExtensionRunName(runner.runName);
    ctx.setExtensionRunInputJSON(runner.runInputJSON);
    ctx.setExtensionSummary(runner.summary);
  }

  async function testAndRegisterExtension(name: string) {
    ctx.setError('');
    try {
      const testResult = await testExtensionByName(name);
      if (testResult.status && testResult.status !== 'passed') {
        ctx.setExtensionSummary(extensionTestSummary(name, `tests ${testResult.status}`));
        return;
      }
      const result = await registerExtensionByName(name);
      ctx.setExtensionSummary(extensionRegisterSummary(name, result.status, 'registered after tests'));
      ctx.pushActivity(`extension test/register: ${name}`);
      await refreshExtensions();
      await reviewExtension(name);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function reuseExtension(name: string) {
    const trimmed = String(name || '').trim();
    if (!trimmed) return;
    ctx.setActiveTab('extensions');
    ctx.setExtensionRunName(trimmed);
    ctx.setExtensionSummary(extensionReuseSummary(trimmed));
    await reviewExtension(trimmed);
  }

  async function proposeExtension() {
    ctx.setError('');
    ctx.setExtensionSummary('');
    ctx.setCapabilityProposal(null);
    ctx.setCapabilityNextSteps([]);
    try {
      const request = ctx.getExtensionProposalRequest();
      const proposal = await proposeCapability(request);
      ctx.setCapabilityProposal(proposal);
      ctx.setCapabilityProposalRequest(request);
      ctx.setExtensionGenerateName(proposal.name || proposal.extension?.name || ctx.getExtensionGenerateName());
      ctx.setExtensionGenerateDescription(proposal.description || proposal.extension?.description || request);
      ctx.setExtensionSummary(
        proposal.canGenerate
          ? `capability proposal: ${proposal.name || proposal.title}`
          : `capability route: ${proposal.title || proposal.kind}`
      );
      ctx.pushActivity(`capability proposal: ${proposal.title || proposal.name}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function generateExtension() {
    ctx.setError('');
    ctx.setExtensionSummary('');
    ctx.setCapabilityNextSteps([]);
    try {
      const request = ctx.getExtensionProposalRequest();
      if (request.trim()) {
        const result = await generateCapability(
          capabilityGenerationInputFromForm({
            request,
            name: ctx.getExtensionGenerateName(),
            proposalRequest: ctx.getCapabilityProposalRequest(),
            proposal: ctx.getCapabilityProposal(),
            runAfterGenerate: ctx.getExtensionGenerateRun(),
            runInputJSON: ctx.getExtensionRunInputJSON(),
            scheduleAfterGenerate: ctx.getExtensionGenerateJob(),
            scheduleType: ctx.getExtensionJobScheduleType(),
            scheduleExpr: ctx.getExtensionJobScheduleExpr(),
            scheduleEnabled: ctx.getExtensionJobEnabled(),
            scheduleInputJSON: ctx.getExtensionJobInputJSON()
          })
        );
        const generatedName = generatedCapabilityName(result, ctx.getExtensionGenerateName());
        ctx.setExtensionSummary(result.message || `generated ${generatedName}`);
        ctx.setExtensionRunName(generatedName);
        if (result.run) {
          ctx.setExtensionRunOutput(previewJSON(result.run));
          ctx.pushActivity(`capability run: ${result.run.extension} ${result.run.status}`);
        }
        if (result.schedule) {
          ctx.setJobSummary(`created ${result.schedule.id}`);
          ctx.pushActivity(`capability job: ${result.schedule.name || result.schedule.id}`);
          await ctx.refreshAutomation();
        }
        ctx.setCapabilityProposal(result.proposal);
        ctx.setCapabilityProposalRequest(request);
        ctx.setCapabilityNextSteps(result.nextSteps ?? []);
        ctx.pushActivity(`capability generated: ${generatedName}`);
      } else {
        const result = await generateExtensionFromDescription({
          name: ctx.getExtensionGenerateName(),
          description: ctx.getExtensionGenerateDescription(),
          approved: true
        });
        const runName = generatedExtensionNameFromLegacyResult(result, ctx.getExtensionGenerateName());
        ctx.setExtensionRunName(runName);
        ctx.setExtensionSummary(`generated ${runName}`);
        ctx.pushActivity(`extension generated: ${runName}`);
      }
      await refreshExtensions();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function validateExtension(name: string) {
    ctx.setError('');
    try {
      const result = await validateExtensionByName(name);
      ctx.setExtensionSummary(extensionActionSummary(result));
      ctx.pushActivity(`extension valid: ${result.extension.name}`);
      ctx.setExtensions(await listExtensions());
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function testExtension(name: string) {
    ctx.setError('');
    try {
      const result = await testExtensionByName(name);
      ctx.setExtensionSummary(extensionTestSummary(name, result.status));
      ctx.pushActivity(`extension tested: ${name}`);
      ctx.setExtensionFailures(await listExtensionFailures(20));
      if (ctx.getExtensionReview()?.detail?.status?.name === name) await reviewExtension(name);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function registerExtension(name: string) {
    ctx.setError('');
    try {
      const result = await registerExtensionByName(name);
      ctx.setExtensionSummary(extensionRegisterSummary(name, result.status));
      ctx.pushActivity(`extension registered: ${name}`);
      await refreshExtensions();
      await reviewExtension(name);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function setExtensionEnabled(name: string, enabled: boolean) {
    ctx.setError('');
    try {
      const result = await setExtensionEnabledState(name, enabled);
      ctx.setExtensionSummary(extensionActionSummary(result));
      ctx.pushActivity(`extension ${result.message}: ${result.extension.name}`);
      ctx.setExtensions(await listExtensions());
      if (ctx.getExtensionReview()?.detail?.status?.name === name) await reviewExtension(name);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function deleteExtension(name: string) {
    ctx.setError('');
    try {
      const result = await deleteExtensionByName(name);
      ctx.setExtensionSummary(extensionActionSummary(result));
      ctx.pushActivity(`extension deleted: ${result.extension.name}`);
      if (ctx.getExtensionReview()?.detail?.status?.name === name) {
        ctx.setExtensionReview(null);
        ctx.setExtensionReviewOpenFile('');
      }
      await refreshExtensions();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function runExtension() {
    ctx.setError('');
    ctx.setExtensionRunOutput('');
    try {
      const input = extensionRunInputFromJSON(ctx.getExtensionRunInputJSON());
      const result = await runExtensionByName(ctx.getExtensionRunName(), input);
      ctx.setExtensionRunOutput(previewJSON(result));
      ctx.setExtensionSummary(extensionRunSummary(result));
      ctx.pushActivity(`extension run: ${result.extension} ${result.status}`);
      ctx.setExtensionFailures(await listExtensionFailures(20));
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function rollbackExtension() {
    ctx.setError('');
    ctx.setExtensionSummary('');
    try {
      const result = await rollbackExtensionTarget(ctx.getExtensionRollbackTarget());
      ctx.setExtensionSummary(result.message || 'rollback completed');
      ctx.setExtensionRollbackTarget('');
      ctx.pushActivity(`extension rollback completed: ${result.name || 'snapshot'}`);
      await refreshExtensions();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    refreshExtensions,
    reviewExtension,
    useReviewedExtensionInRunner,
    testAndRegisterExtension,
    reuseExtension,
    proposeExtension,
    generateExtension,
    validateExtension,
    testExtension,
    registerExtension,
    setExtensionEnabled,
    deleteExtension,
    runExtension,
    rollbackExtension
  };
}
