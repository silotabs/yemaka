import type { ExtensionReview, ExtensionRunResult } from './appTypes';
import { parseSchedulerJobInput, schedulerJobRunOutputSummary } from './schedulerJobHelpers';
import { humanizeIdentifier } from './uiHelpers';

export type ExtensionReviewFormState = {
  reviewOpenFile: string;
  runName: string;
  runInputJSON: string;
  summary: string;
  activity: string;
};

export type ExtensionRunnerState = {
  runName: string;
  runInputJSON: string;
  summary: string;
};

export function extensionReviewFormState(review: ExtensionReview, requestedName: string, currentInputJSON: string): ExtensionReviewFormState {
  const runName = review.detail?.status?.name || requestedName;
  return {
    reviewOpenFile: review.files?.[0]?.path || '',
    runName,
    runInputJSON: review.sampleInputJson || currentInputJSON,
    summary: `${runName}: review ready`,
    activity: `extension review: ${runName}`
  };
}

export function reviewedExtensionRunnerState(review: ExtensionReview, currentRunName: string, currentInputJSON: string): ExtensionRunnerState {
  const runName = review.detail?.status?.name || currentRunName;
  return {
    runName,
    runInputJSON: review.sampleInputJson || currentInputJSON,
    summary: `${runName}: runner input prepared`
  };
}

export function extensionTestSummary(name: string, status: string | undefined, fallback = 'tested') {
  return `${name}: ${status || fallback}`;
}

export function extensionRegisterSummary(name: string, status: string | undefined, fallback = 'registered') {
  return `${name}: ${status || fallback}`;
}

export function extensionReuseSummary(name: string) {
  return `${name} selected for reuse`;
}

export function extensionRunInputFromJSON(value: string): Record<string, unknown> {
  return parseSchedulerJobInput(value, 'Run input');
}

export function extensionRunSummary(result: ExtensionRunResult) {
  const summary = result.error || schedulerJobRunOutputSummary(result.output);
  return `${humanizeIdentifier(result.extension, result.extension)}: ${humanizeIdentifier(result.status, result.status)} - ${summary}`;
}
