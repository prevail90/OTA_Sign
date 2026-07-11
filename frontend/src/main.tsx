import React from 'react';
import { createRoot } from 'react-dom/client';
import {
  ChevronDown,
  ChevronRight,
  Download,
  ExternalLink,
  FileSignature,
  Monitor,
  Moon,
  Search,
  ShieldCheck,
  Sun,
  Trash2,
} from 'lucide-react';
import './styles.css';

const runtimeConfig = window.__OTASIGN_CONFIG__ ?? {};
const API_BASE = runtimeConfig.apiBaseUrl ?? import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080';
const MOODLE_LOGIN_URL =
  runtimeConfig.moodleLoginUrl ?? import.meta.env.VITE_MOODLE_LOGIN_URL ?? 'http://localhost:8081/login/index.php';
const MOODLE_LAUNCH_URL =
  runtimeConfig.moodleLaunchUrl ?? import.meta.env.VITE_MOODLE_LAUNCH_URL ?? MOODLE_LOGIN_URL;
const REFRESH_INTERVAL_MS = 12000;
const THEME_STORAGE_KEY = 'otasign-theme';

type ThemePreference = 'system' | 'light' | 'dark';

type User = {
  id: string;
  moodle_user_id: string;
  full_name: string;
  email: string;
  dod_id?: string;
  uic: string;
  roles: string[];
  capabilities: string[];
};

type Template = {
  id: string;
  name: string;
  docuseal_template_id: string;
  requires_commander_signature: boolean;
  commander_role_name?: string;
};

type Submission = {
  id: string;
  template_id: string;
  template_name: string;
  soldier_user_id: string;
  soldier_name: string;
  soldier_dod_id?: string;
  uic: string;
  status: 'missing' | 'pending' | 'complete' | 'canceled' | 'failed';
  current: boolean;
  requires_commander: boolean;
  waiting_on_commander: boolean;
  docuseal_submission_id?: string;
  signing_url?: string;
  created_at?: string;
  completed_at?: string;
};

type SoldierSubmissionGroup = {
  soldier_user_id: string;
  soldier_name: string;
  soldier_dod_id?: string;
  uic: string;
  submissions: Submission[];
};

function App() {
  const [user, setUser] = React.useState<User | null>(null);
  const [templates, setTemplates] = React.useState<Template[]>([]);
  const [mySubmissions, setMySubmissions] = React.useState<Submission[]>([]);
  const [unitGroups, setUnitGroups] = React.useState<SoldierSubmissionGroup[]>([]);
  const [activeTab, setActiveTab] = React.useState<'my' | 'unit'>('my');
  const [search, setSearch] = React.useState('');
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [actionError, setActionError] = React.useState<string | null>(null);
  const [busyTemplateID, setBusyTemplateID] = React.useState<string | null>(null);
  const [busySubmissionID, setBusySubmissionID] = React.useState<string | null>(null);
  const [busyDownloadSubmissionID, setBusyDownloadSubmissionID] = React.useState<string | null>(null);
  const [busyDownloadSoldierID, setBusyDownloadSoldierID] = React.useState<string | null>(null);
  const [busyCommanderSubmissionID, setBusyCommanderSubmissionID] = React.useState<string | null>(null);
  const [themePreference, setThemePreference] = React.useState<ThemePreference>(() => readThemePreference());

  const canViewUnit = user?.capabilities.includes('viewunit') ?? false;
  const canSignCommander = user?.capabilities.includes('signascommander') ?? false;

  React.useEffect(() => {
    applyThemePreference(themePreference);
    window.localStorage.setItem(THEME_STORAGE_KEY, themePreference);
  }, [themePreference]);

  React.useEffect(() => {
    Promise.all([
      api<User>('/api/me'),
      api<Template[]>('/api/templates'),
      api<Submission[]>('/api/my/submissions'),
    ])
      .then(([nextUser, nextTemplates, nextSubmissions]) => {
        setUser(nextUser);
        setTemplates(nextTemplates);
        setMySubmissions(nextSubmissions);
      })
      .catch(() => setError('You need to launch OTA Sign from Moodle.'))
      .finally(() => setLoading(false));
  }, []);

  const refreshMySubmissions = React.useCallback(() => {
    return api<Submission[]>('/api/my/submissions').then(setMySubmissions);
  }, []);

  const refreshUnitSubmissions = React.useCallback(
    (signal?: AbortSignal) => {
      if (!canViewUnit) {
        return Promise.resolve();
      }

      return api<SoldierSubmissionGroup[]>(`/api/unit/submissions?search=${encodeURIComponent(search)}`, signal).then(
        setUnitGroups,
      );
    },
    [canViewUnit, search],
  );

  React.useEffect(() => {
    if (!canViewUnit) {
      return;
    }

    const controller = new AbortController();
    refreshUnitSubmissions(controller.signal).catch(() => undefined);

    return () => controller.abort();
  }, [canViewUnit, refreshUnitSubmissions]);

  React.useEffect(() => {
    if (!user) {
      return;
    }

    const refreshActiveTab = () => {
      if (document.visibilityState === 'hidden') {
        return;
      }

      if (activeTab === 'unit') {
        refreshUnitSubmissions().catch(() => undefined);
        return;
      }

      refreshMySubmissions().catch(() => undefined);
    };

    const intervalID = window.setInterval(refreshActiveTab, REFRESH_INTERVAL_MS);
    window.addEventListener('focus', refreshActiveTab);
    document.addEventListener('visibilitychange', refreshActiveTab);

    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener('focus', refreshActiveTab);
      document.removeEventListener('visibilitychange', refreshActiveTab);
    };
  }, [activeTab, refreshMySubmissions, refreshUnitSubmissions, user]);

  const startSubmission = React.useCallback(
    (templateID: string) => {
      setActionError(null);
      setBusyTemplateID(templateID);

      api<Submission>('/api/submissions', undefined, {
        method: 'POST',
        body: JSON.stringify({ template_id: templateID }),
      })
        .then((submission) => {
          if (submission.signing_url) {
            window.open(submission.signing_url, '_blank', 'noopener,noreferrer');
          }
          return Promise.all([refreshMySubmissions(), refreshUnitSubmissions()]);
        })
        .catch((err) => setActionError(err instanceof Error ? err.message : 'Could not start submission.'))
        .finally(() => setBusyTemplateID(null));
    },
    [refreshMySubmissions, refreshUnitSubmissions],
  );

  const deleteSubmission = React.useCallback(
    (submissionID: string) => {
      if (!window.confirm('Delete this submission?')) {
        return;
      }

      setActionError(null);
      setBusySubmissionID(submissionID);

      api<{ status: string }>(`/api/submissions/${encodeURIComponent(submissionID)}`, undefined, {
        method: 'DELETE',
      })
        .then(() => Promise.all([refreshMySubmissions(), refreshUnitSubmissions()]))
        .catch((err) => setActionError(err instanceof Error ? err.message : 'Could not delete submission.'))
        .finally(() => setBusySubmissionID(null));
    },
    [refreshMySubmissions, refreshUnitSubmissions],
  );

  const downloadSubmission = React.useCallback((submissionID: string) => {
    setActionError(null);
    setBusyDownloadSubmissionID(submissionID);

    downloadFile(`/api/submissions/${encodeURIComponent(submissionID)}/download`)
      .catch((err) => setActionError(err instanceof Error ? err.message : 'Could not download submission.'))
      .finally(() => setBusyDownloadSubmissionID(null));
  }, []);

  const viewSubmission = React.useCallback((submissionID: string) => {
    window.open(`${API_BASE}/api/submissions/${encodeURIComponent(submissionID)}/view`, '_blank', 'noopener,noreferrer');
  }, []);

  const downloadSoldierSubmissions = React.useCallback((soldierUserID: string) => {
    setActionError(null);
    setBusyDownloadSoldierID(soldierUserID);

    downloadFile(`/api/unit/soldiers/${encodeURIComponent(soldierUserID)}/download`)
      .catch((err) => setActionError(err instanceof Error ? err.message : 'Could not download soldier forms.'))
      .finally(() => setBusyDownloadSoldierID(null));
  }, []);

  const signAsCommander = React.useCallback(
    (submissionID: string) => {
      setActionError(null);
      setBusyCommanderSubmissionID(submissionID);

      api<{ signing_url: string }>(`/api/submissions/${encodeURIComponent(submissionID)}/commander-sign`, undefined, {
        method: 'POST',
      })
        .then((response) => {
          window.open(response.signing_url, '_blank', 'noopener,noreferrer');
          return refreshUnitSubmissions();
        })
        .catch((err) => setActionError(err instanceof Error ? err.message : 'Could not prepare commander signature.'))
        .finally(() => setBusyCommanderSubmissionID(null));
    },
    [refreshUnitSubmissions],
  );

  if (loading) {
    return <main className="centered">Loading OTA Sign...</main>;
  }

  if (error || !user) {
    return (
      <main className="centered">
        <section className="emptyState">
          <ShieldCheck size={34} />
          <h1>Launch Required</h1>
          <p>{error}</p>
          <a className="primaryLink" href={MOODLE_LAUNCH_URL}>
            Launch From Moodle
          </a>
        </section>
      </main>
    );
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">OTA Sign</p>
          <h1>Forms Portal</h1>
        </div>
        <div className="topActions">
          <ThemeControl themePreference={themePreference} setThemePreference={setThemePreference} />
          <div className="identity">
            <strong>{user.full_name}</strong>
            <span>{user.uic}</span>
          </div>
        </div>
      </header>

      <nav className="tabs" aria-label="Dashboard views">
        <button className={activeTab === 'my' ? 'active' : ''} onClick={() => setActiveTab('my')}>
          My Forms
        </button>
        {canViewUnit && (
          <button className={activeTab === 'unit' ? 'active' : ''} onClick={() => setActiveTab('unit')}>
            Unit Forms
          </button>
        )}
      </nav>

      {actionError && <p className="errorText">{actionError}</p>}

      {activeTab === 'my' ? (
        <MyForms
          templates={templates}
          submissions={mySubmissions}
          busyTemplateID={busyTemplateID}
          busySubmissionID={busySubmissionID}
          busyDownloadSubmissionID={busyDownloadSubmissionID}
          startSubmission={startSubmission}
          deleteSubmission={deleteSubmission}
          downloadSubmission={downloadSubmission}
        />
      ) : (
        <UnitForms
          groups={unitGroups}
          search={search}
          setSearch={setSearch}
          canSignCommander={canSignCommander}
          busyDownloadSubmissionID={busyDownloadSubmissionID}
          busyDownloadSoldierID={busyDownloadSoldierID}
          busyCommanderSubmissionID={busyCommanderSubmissionID}
          signAsCommander={signAsCommander}
          viewSubmission={viewSubmission}
          downloadSubmission={downloadSubmission}
          downloadSoldierSubmissions={downloadSoldierSubmissions}
        />
      )}
    </main>
  );
}

function ThemeControl({
  themePreference,
  setThemePreference,
}: {
  themePreference: ThemePreference;
  setThemePreference: (themePreference: ThemePreference) => void;
}) {
  return (
    <div className="themeControl" aria-label="Theme preference">
      <button
        aria-label="Use system theme"
        className={themePreference === 'system' ? 'active' : ''}
        title="System"
        onClick={() => setThemePreference('system')}
      >
        <Monitor size={16} />
      </button>
      <button
        aria-label="Use light theme"
        className={themePreference === 'light' ? 'active' : ''}
        title="Light"
        onClick={() => setThemePreference('light')}
      >
        <Sun size={16} />
      </button>
      <button
        aria-label="Use dark theme"
        className={themePreference === 'dark' ? 'active' : ''}
        title="Dark"
        onClick={() => setThemePreference('dark')}
      >
        <Moon size={16} />
      </button>
    </div>
  );
}

function readThemePreference(): ThemePreference {
  const stored = window.localStorage.getItem(THEME_STORAGE_KEY);
  if (stored === 'light' || stored === 'dark' || stored === 'system') {
    return stored;
  }
  return 'system';
}

function applyThemePreference(themePreference: ThemePreference) {
  const root = document.documentElement;
  if (themePreference === 'system') {
    root.removeAttribute('data-theme');
    return;
  }
  root.dataset.theme = themePreference;
}

function MyForms({
  templates,
  submissions,
  busyTemplateID,
  busySubmissionID,
  busyDownloadSubmissionID,
  startSubmission,
  deleteSubmission,
  downloadSubmission,
}: {
  templates: Template[];
  submissions: Submission[];
  busyTemplateID: string | null;
  busySubmissionID: string | null;
  busyDownloadSubmissionID: string | null;
  startSubmission: (templateID: string) => void;
  deleteSubmission: (submissionID: string) => void;
  downloadSubmission: (submissionID: string) => void;
}) {
  return (
    <section className="panel">
      <div className="panelHeader">
        <div>
          <h2>Available Forms</h2>
          <p>Current templates and your latest submission status.</p>
        </div>
      </div>

      <div className="formGrid">
        {templates.map((template) => {
          const matching = submissions.filter((submission) => submission.template_id === template.id);
          const current =
            matching.find((submission) => submission.current) ??
            matching.find((submission) => submission.status === 'complete');
          const docusealSubmissionCount = matching.filter(
            (submission) =>
              submission.docuseal_submission_id &&
              submission.status !== 'missing' &&
              submission.status !== 'canceled' &&
              submission.status !== 'failed',
          ).length;

          return (
            <article className="formCard" key={template.id}>
              <div>
                <h3>{template.name}</h3>
                <StatusPill status={current?.status ?? 'missing'} />
              </div>
              <p>{template.requires_commander_signature ? 'Commander signature required' : 'Soldier signature only'}</p>
              <div className="buttonRow">
                {current?.status === 'complete' && (
                  <button
                    className="iconButton"
                    disabled={busyDownloadSubmissionID === current.id}
                    onClick={() => downloadSubmission(current.id)}
                  >
                    <Download size={16} />
                    {busyDownloadSubmissionID === current.id ? 'Downloading...' : 'Download'}
                  </button>
                )}
                {current?.status === 'pending' && (
                  <button
                    className="iconButton"
                    disabled={!current.signing_url}
                    onClick={() => current.signing_url && window.open(current.signing_url, '_blank', 'noopener,noreferrer')}
                  >
                    <ExternalLink size={16} />
                    View
                  </button>
                )}
                {current && current.status !== 'complete' && current.status !== 'missing' && (
                  <button
                    className="iconButton dangerButton"
                    disabled={busySubmissionID === current.id}
                    onClick={() => deleteSubmission(current.id)}
                  >
                    <Trash2 size={16} />
                    {busySubmissionID === current.id ? 'Deleting...' : 'Delete'}
                  </button>
                )}
                {current?.status !== 'pending' && (
                  <button
                    className="primaryButton"
                    disabled={busyTemplateID === template.id}
                    onClick={() => startSubmission(template.id)}
                  >
                    <FileSignature size={16} />
                    {busyTemplateID === template.id ? 'Starting...' : 'Start New'}
                  </button>
                )}
              </div>
              {docusealSubmissionCount > 0 && (
                <span className="muted">
                  {docusealSubmissionCount} DocuSeal submission{docusealSubmissionCount === 1 ? '' : 's'}
                </span>
              )}
            </article>
          );
        })}
      </div>
    </section>
  );
}

function UnitForms({
  groups,
  search,
  setSearch,
  canSignCommander,
  busyDownloadSubmissionID,
  busyDownloadSoldierID,
  busyCommanderSubmissionID,
  signAsCommander,
  viewSubmission,
  downloadSubmission,
  downloadSoldierSubmissions,
}: {
  groups: SoldierSubmissionGroup[];
  search: string;
  setSearch: (value: string) => void;
  canSignCommander: boolean;
  busyDownloadSubmissionID: string | null;
  busyDownloadSoldierID: string | null;
  busyCommanderSubmissionID: string | null;
  signAsCommander: (submissionID: string) => void;
  viewSubmission: (submissionID: string) => void;
  downloadSubmission: (submissionID: string) => void;
  downloadSoldierSubmissions: (soldierUserID: string) => void;
}) {
  const [collapsed, setCollapsed] = React.useState<Record<string, boolean>>({});

  return (
    <section className="panel">
      <div className="panelHeader unitHeader">
        <div>
          <h2>Unit Forms</h2>
          <p>Most current submissions grouped by soldier.</p>
        </div>
        <label className="searchBox">
          <Search size={18} />
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search name, DoD ID, or form"
          />
        </label>
      </div>

      <div className="soldierList">
        {groups.map((group) => {
          const isCollapsed = collapsed[group.soldier_user_id];
          const completedSubmissions = group.submissions.filter((submission) => submission.status === 'complete');
          return (
            <article className="soldierGroup" key={group.soldier_user_id}>
              <div className="soldierHeader">
                <button
                  className="soldierToggle"
                  onClick={() =>
                    setCollapsed((current) => ({
                      ...current,
                      [group.soldier_user_id]: !current[group.soldier_user_id],
                    }))
                  }
                >
                  {isCollapsed ? <ChevronRight size={20} /> : <ChevronDown size={20} />}
                  <span>{group.soldier_name}</span>
                  <small>{group.soldier_dod_id}</small>
                </button>
                {completedSubmissions.length > 0 && (
                  <button
                    className="iconButton"
                    disabled={busyDownloadSoldierID === group.soldier_user_id}
                    onClick={() => downloadSoldierSubmissions(group.soldier_user_id)}
                  >
                    <Download size={16} />
                    {busyDownloadSoldierID === group.soldier_user_id ? 'Downloading...' : 'Download all'}
                  </button>
                )}
              </div>
              {!isCollapsed && (
                <div className="submissionTable">
                  {group.submissions.map((submission) => (
                    <div className="submissionRow" key={submission.id}>
                      <div>
                        <strong>{submission.template_name}</strong>
                        <span>{submission.waiting_on_commander ? 'Waiting on commander signature' : 'Current status'}</span>
                      </div>
                      <StatusPill status={submission.status} />
                      <div className="rowActions">
                        {canSignCommander && submission.waiting_on_commander && (
                          <button
                            className="primaryButton"
                            disabled={busyCommanderSubmissionID === submission.id}
                            onClick={() => signAsCommander(submission.id)}
                          >
                            <FileSignature size={16} />
                            {busyCommanderSubmissionID === submission.id ? 'Preparing...' : 'Sign'}
                          </button>
                        )}
                        {submission.status === 'complete' && (
                          <>
                            <button className="iconButton" onClick={() => viewSubmission(submission.id)}>
                              <ExternalLink size={16} />
                              View
                            </button>
                            <button
                              className="iconButton"
                              disabled={busyDownloadSubmissionID === submission.id}
                              onClick={() => downloadSubmission(submission.id)}
                            >
                              <Download size={16} />
                              {busyDownloadSubmissionID === submission.id ? 'Downloading...' : 'Download'}
                            </button>
                          </>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </article>
          );
        })}
      </div>
    </section>
  );
}

function StatusPill({ status }: { status: Submission['status'] }) {
  return <span className={`status ${status}`}>{status}</span>;
}

async function api<T>(path: string, signal?: AbortSignal, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    signal,
    headers: {
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  });

  if (!response.ok) {
    let message = `Request failed: ${response.status}`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) {
        message = body.error;
      }
    } catch {
      // Keep the generic HTTP status message.
    }
    throw new Error(message);
  }

  return response.json() as Promise<T>;
}

async function downloadFile(path: string): Promise<void> {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
  });

  if (!response.ok) {
    let message = `Request failed: ${response.status}`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) {
        message = body.error;
      }
    } catch {
      // Keep the generic HTTP status message.
    }
    throw new Error(message);
  }

  const blob = await response.blob();
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filenameFromDisposition(response.headers.get('Content-Disposition'));
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);
}

function filenameFromDisposition(disposition: string | null): string {
  const match = disposition?.match(/filename="([^"]+)"/i);
  return match?.[1] ?? 'otasign-submission.pdf';
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
