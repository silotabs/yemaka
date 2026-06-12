export type ToastKind = 'success' | 'error' | 'warning' | 'info';

export type ToastItem = {
  id: number;
  kind: ToastKind;
  title: string;
  message: string;
};

export function toastKindForMessage(message: string): ToastKind {
  const lower = String(message || '').toLowerCase();
  if (/\b(failed|failure|error|crashed|broken|rejected)\b/.test(lower)) return 'error';
  if (/\b(cannot|blocked|invalid|missing|required|needs|skipped|warning)\b/.test(lower)) return 'warning';
  if (/\b(saved|created|installed|enabled|disabled|archived|generated|imported|exported|applied|registered|tested|loaded|completed|refreshed|ran|renamed|deleted|starred|unstarred)\b/.test(lower)) {
    return 'success';
  }
  return 'info';
}

export function toastTitle(kind: ToastKind) {
  if (kind === 'success') return 'Done';
  if (kind === 'error') return 'Error';
  if (kind === 'warning') return 'Needs attention';
  return 'Yemaka';
}

export function toastIcon(kind: ToastKind) {
  if (kind === 'success') return 'success';
  if (kind === 'error') return 'error';
  if (kind === 'warning') return 'warning';
  return 'info';
}

export function toastTimeout(kind: ToastKind) {
  if (kind === 'error') return 8000;
  if (kind === 'warning') return 7000;
  return 4800;
}
