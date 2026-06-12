export const activeTabStorageKey = 'yemaka.activeTab';
export const activeConversationStorageKey = 'yemaka.activeConversationId';
export const sidebarVisibleStorageKey = 'yemaka.sidebarVisible';

export type StoredNavigation<TTab extends string> = {
  tab?: TTab;
  conversationId: string;
};

export function persistActiveTab(tab: string) {
  localStorage.setItem(activeTabStorageKey, tab);
}

export function persistActiveConversation(conversationId: string) {
  if (conversationId) {
    localStorage.setItem(activeConversationStorageKey, conversationId);
  } else {
    localStorage.removeItem(activeConversationStorageKey);
  }
}

export function restoreStoredNavigation<TTab extends string>(isTab: (value: string | null) => value is TTab): StoredNavigation<TTab> {
  const storedTab = localStorage.getItem(activeTabStorageKey);
  let tab: TTab | undefined;
  if (isTab(storedTab)) {
    tab = storedTab;
  } else if (storedTab) {
    localStorage.removeItem(activeTabStorageKey);
  }
  return {
    tab,
    conversationId: localStorage.getItem(activeConversationStorageKey) || ''
  };
}

export function persistSidebarVisible(value: boolean) {
  localStorage.setItem(sidebarVisibleStorageKey, value ? 'true' : 'false');
}

export function restoreSidebarVisible() {
  return localStorage.getItem(sidebarVisibleStorageKey) !== 'false';
}
