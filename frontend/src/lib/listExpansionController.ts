import type { ToolRun } from './appTypes';
import {
  expandedByDefault,
  toggledExpandedState,
  type ToolRunConversationGroup
} from './toolRunHelpers';

type ListExpansionControllerContext = {
  getListPages: () => Record<string, number>;
  setListPages: (pages: Record<string, number>) => void;
  getExpandedToolRunGroups: () => Record<string, boolean>;
  setExpandedToolRunGroups: (state: Record<string, boolean>) => void;
  getExpandedToolRunSessions: () => Record<string, boolean>;
  setExpandedToolRunSessions: (state: Record<string, boolean>) => void;
  getGroupedToolRuns: () => Array<ToolRunConversationGroup<ToolRun>>;
};

export function createListExpansionController(ctx: ListExpansionControllerContext) {
  function setListPage(key: string, page: number) {
    ctx.setListPages({ ...ctx.getListPages(), [key]: Math.max(1, page) });
  }

  function toolRunGroupExpanded(id: string, total: number, expandedState = ctx.getExpandedToolRunGroups()) {
    return expandedByDefault(id, total, expandedState, false);
  }

  function toolRunSessionExpanded(id: string, total: number, expandedState = ctx.getExpandedToolRunSessions()) {
    return expandedByDefault(id, total, expandedState, false);
  }

  function toggleToolRunGroup(id: string) {
    const expandedGroups = ctx.getExpandedToolRunGroups();
    const current = expandedGroups[id] ?? ctx.getGroupedToolRuns().length <= 1;
    ctx.setExpandedToolRunGroups(toggledExpandedState(id, expandedGroups, current));
  }

  function toggleToolRunSession(id: string) {
    const expandedSessions = ctx.getExpandedToolRunSessions();
    const current = expandedSessions[id] ?? false;
    ctx.setExpandedToolRunSessions(toggledExpandedState(id, expandedSessions, current));
  }

  return {
    setListPage,
    toolRunGroupExpanded,
    toolRunSessionExpanded,
    toggleToolRunGroup,
    toggleToolRunSession
  };
}
