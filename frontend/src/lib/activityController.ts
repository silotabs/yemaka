import {
  activityEntriesFromToolRuns,
  createActivityEntry,
  prependActivityEntry,
  shouldHydrateActivity
} from './activityHelpers';
import type { ActivityEntry, ToolRun } from './appTypes';
import { formatShortTime, toolRunTimestamp } from './toolRunHelpers';

type ActivityControllerContext = {
  getActivity: () => ActivityEntry[];
  setActivity: (items: ActivityEntry[]) => void;
  getActivitySequence: () => number;
  setActivitySequence: (value: number) => void;
  getToolRuns: () => ToolRun[];
};

export function createActivityController(ctx: ActivityControllerContext) {
  function pushActivity(line: string) {
    const nextSequence = ctx.getActivitySequence() + 1;
    const entry = createActivityEntry(line, nextSequence);
    if (!entry) return;
    ctx.setActivitySequence(nextSequence);
    ctx.setActivity(prependActivityEntry(ctx.getActivity(), entry));
  }

  function hydrateActivityFromToolRuns() {
    const activity = ctx.getActivity();
    const toolRuns = ctx.getToolRuns();
    if (!shouldHydrateActivity(activity, toolRuns)) return;
    ctx.setActivity(activityEntriesFromToolRuns(toolRuns, toolRunTimestamp, formatShortTime));
  }

  return {
    pushActivity,
    hydrateActivityFromToolRuns
  };
}
