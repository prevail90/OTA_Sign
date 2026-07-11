CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    moodle_user_id TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    middle_initial TEXT,
    email TEXT NOT NULL,
    army_email TEXT,
    dod_id TEXT,
    rank TEXT,
    pay_grade TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS middle_initial TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS army_email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS pay_grade TEXT;

CREATE TABLE IF NOT EXISTS dod_paygrades (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_branch TEXT NOT NULL,
    pay_grade TEXT NOT NULL,
    rank_abbreviation TEXT NOT NULL,
    rank_title TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (service_branch, pay_grade)
);

CREATE TABLE IF NOT EXISTS units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uic TEXT NOT NULL UNIQUE,
    name TEXT,
    component TEXT,
    region TEXT,
    country TEXT,
    installation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_unit_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    unit_id UUID NOT NULL REFERENCES units(id),
    role TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'moodle',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, unit_id, role)
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    roles JSONB NOT NULL DEFAULT '[]'::jsonb,
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE IF NOT EXISTS templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    docuseal_template_id TEXT NOT NULL UNIQUE,
    docuseal_template_slug TEXT,
    soldier_role_name TEXT NOT NULL DEFAULT 'Soldier',
    name TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    requires_commander_signature BOOLEAN NOT NULL DEFAULT false,
    commander_role_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE templates ADD COLUMN IF NOT EXISTS docuseal_template_slug TEXT;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS soldier_role_name TEXT NOT NULL DEFAULT 'Soldier';

CREATE TABLE IF NOT EXISTS template_unit_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES templates(id),
    unit_id UUID REFERENCES units(id),
    region TEXT,
    country TEXT,
    installation TEXT,
    include BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id UUID NOT NULL REFERENCES templates(id),
    soldier_user_id UUID NOT NULL REFERENCES users(id),
    unit_id UUID NOT NULL REFERENCES units(id),
    docuseal_submission_id TEXT UNIQUE,
    status TEXT NOT NULL,
    current BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS submission_signers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES submissions(id),
    user_id UUID REFERENCES users(id),
    role TEXT NOT NULL,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    docuseal_submitter_id TEXT,
    signing_url TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stored_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES submissions(id),
    storage_key TEXT NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/pdf',
    size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS docuseal_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id TEXT UNIQUE,
    event_type TEXT NOT NULL,
    docuseal_submission_id TEXT,
    payload JSONB NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS commander_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    unit_id UUID NOT NULL REFERENCES units(id),
    submitted_name TEXT NOT NULL,
    submitted_email TEXT NOT NULL,
    pdf_storage_key TEXT,
    extracted_pdf_text TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    approved_at TIMESTAMPTZ,
    denied_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO dod_paygrades (service_branch, pay_grade, rank_abbreviation, rank_title, sort_order) VALUES
    ('Army', 'E-1', 'PVT', 'Private', 1),
    ('Army', 'E-2', 'PV2', 'Private Second Class', 2),
    ('Army', 'E-3', 'PFC', 'Private First Class', 3),
    ('Army', 'E-4', 'SPC', 'Specialist', 4),
    ('Army', 'E-5', 'SGT', 'Sergeant', 5),
    ('Army', 'E-6', 'SSG', 'Staff Sergeant', 6),
    ('Army', 'E-7', 'SFC', 'Sergeant First Class', 7),
    ('Army', 'E-8', 'MSG', 'Master Sergeant', 8),
    ('Army', 'E-9', 'SGM', 'Sergeant Major', 9),
    ('Army', 'W-1', 'WO1', 'Warrant Officer 1', 10),
    ('Army', 'W-2', 'CW2', 'Chief Warrant Officer 2', 11),
    ('Army', 'W-3', 'CW3', 'Chief Warrant Officer 3', 12),
    ('Army', 'W-4', 'CW4', 'Chief Warrant Officer 4', 13),
    ('Army', 'W-5', 'CW5', 'Chief Warrant Officer 5', 14),
    ('Army', 'O-1', '2LT', 'Second Lieutenant', 15),
    ('Army', 'O-2', '1LT', 'First Lieutenant', 16),
    ('Army', 'O-3', 'CPT', 'Captain', 17),
    ('Army', 'O-4', 'MAJ', 'Major', 18),
    ('Army', 'O-5', 'LTC', 'Lieutenant Colonel', 19),
    ('Army', 'O-6', 'COL', 'Colonel', 20),
    ('Army', 'O-7', 'BG', 'Brigadier General', 21),
    ('Army', 'O-8', 'MG', 'Major General', 22),
    ('Army', 'O-9', 'LTG', 'Lieutenant General', 23),
    ('Army', 'O-10', 'GEN', 'General', 24),
    ('Marine Corps', 'E-1', 'Pvt', 'Private', 1),
    ('Marine Corps', 'E-2', 'PFC', 'Private First Class', 2),
    ('Marine Corps', 'E-3', 'LCpl', 'Lance Corporal', 3),
    ('Marine Corps', 'E-4', 'Cpl', 'Corporal', 4),
    ('Marine Corps', 'E-5', 'Sgt', 'Sergeant', 5),
    ('Marine Corps', 'E-6', 'SSgt', 'Staff Sergeant', 6),
    ('Marine Corps', 'E-7', 'GySgt', 'Gunnery Sergeant', 7),
    ('Marine Corps', 'E-8', 'MSgt', 'Master Sergeant', 8),
    ('Marine Corps', 'E-9', 'SgtMaj', 'Sergeant Major', 9),
    ('Marine Corps', 'W-1', 'WO', 'Warrant Officer', 10),
    ('Marine Corps', 'W-2', 'CWO2', 'Chief Warrant Officer 2', 11),
    ('Marine Corps', 'W-3', 'CWO3', 'Chief Warrant Officer 3', 12),
    ('Marine Corps', 'W-4', 'CWO4', 'Chief Warrant Officer 4', 13),
    ('Marine Corps', 'W-5', 'CWO5', 'Chief Warrant Officer 5', 14),
    ('Marine Corps', 'O-1', '2ndLt', 'Second Lieutenant', 15),
    ('Marine Corps', 'O-2', '1stLt', 'First Lieutenant', 16),
    ('Marine Corps', 'O-3', 'Capt', 'Captain', 17),
    ('Marine Corps', 'O-4', 'Maj', 'Major', 18),
    ('Marine Corps', 'O-5', 'LtCol', 'Lieutenant Colonel', 19),
    ('Marine Corps', 'O-6', 'Col', 'Colonel', 20),
    ('Marine Corps', 'O-7', 'BGen', 'Brigadier General', 21),
    ('Marine Corps', 'O-8', 'MajGen', 'Major General', 22),
    ('Marine Corps', 'O-9', 'LtGen', 'Lieutenant General', 23),
    ('Marine Corps', 'O-10', 'Gen', 'General', 24),
    ('Navy', 'E-1', 'SR', 'Seaman Recruit', 1),
    ('Navy', 'E-2', 'SA', 'Seaman Apprentice', 2),
    ('Navy', 'E-3', 'SN', 'Seaman', 3),
    ('Navy', 'E-4', 'PO3', 'Petty Officer Third Class', 4),
    ('Navy', 'E-5', 'PO2', 'Petty Officer Second Class', 5),
    ('Navy', 'E-6', 'PO1', 'Petty Officer First Class', 6),
    ('Navy', 'E-7', 'CPO', 'Chief Petty Officer', 7),
    ('Navy', 'E-8', 'SCPO', 'Senior Chief Petty Officer', 8),
    ('Navy', 'E-9', 'MCPO', 'Master Chief Petty Officer', 9),
    ('Navy', 'W-2', 'CWO2', 'Chief Warrant Officer 2', 11),
    ('Navy', 'W-3', 'CWO3', 'Chief Warrant Officer 3', 12),
    ('Navy', 'W-4', 'CWO4', 'Chief Warrant Officer 4', 13),
    ('Navy', 'W-5', 'CWO5', 'Chief Warrant Officer 5', 14),
    ('Navy', 'O-1', 'ENS', 'Ensign', 15),
    ('Navy', 'O-2', 'LTJG', 'Lieutenant Junior Grade', 16),
    ('Navy', 'O-3', 'LT', 'Lieutenant', 17),
    ('Navy', 'O-4', 'LCDR', 'Lieutenant Commander', 18),
    ('Navy', 'O-5', 'CDR', 'Commander', 19),
    ('Navy', 'O-6', 'CAPT', 'Captain', 20),
    ('Navy', 'O-7', 'RDML', 'Rear Admiral Lower Half', 21),
    ('Navy', 'O-8', 'RADM', 'Rear Admiral Upper Half', 22),
    ('Navy', 'O-9', 'VADM', 'Vice Admiral', 23),
    ('Navy', 'O-10', 'ADM', 'Admiral', 24),
    ('Air Force', 'E-1', 'AB', 'Airman Basic', 1),
    ('Air Force', 'E-2', 'Amn', 'Airman', 2),
    ('Air Force', 'E-3', 'A1C', 'Airman First Class', 3),
    ('Air Force', 'E-4', 'SrA', 'Senior Airman', 4),
    ('Air Force', 'E-5', 'SSgt', 'Staff Sergeant', 5),
    ('Air Force', 'E-6', 'TSgt', 'Technical Sergeant', 6),
    ('Air Force', 'E-7', 'MSgt', 'Master Sergeant', 7),
    ('Air Force', 'E-8', 'SMSgt', 'Senior Master Sergeant', 8),
    ('Air Force', 'E-9', 'CMSgt', 'Chief Master Sergeant', 9),
    ('Air Force', 'O-1', '2d Lt', 'Second Lieutenant', 15),
    ('Air Force', 'O-2', '1st Lt', 'First Lieutenant', 16),
    ('Air Force', 'O-3', 'Capt', 'Captain', 17),
    ('Air Force', 'O-4', 'Maj', 'Major', 18),
    ('Air Force', 'O-5', 'Lt Col', 'Lieutenant Colonel', 19),
    ('Air Force', 'O-6', 'Col', 'Colonel', 20),
    ('Air Force', 'O-7', 'Brig Gen', 'Brigadier General', 21),
    ('Air Force', 'O-8', 'Maj Gen', 'Major General', 22),
    ('Air Force', 'O-9', 'Lt Gen', 'Lieutenant General', 23),
    ('Air Force', 'O-10', 'Gen', 'General', 24),
    ('Space Force', 'E-1', 'Spc1', 'Specialist 1', 1),
    ('Space Force', 'E-2', 'Spc2', 'Specialist 2', 2),
    ('Space Force', 'E-3', 'Spc3', 'Specialist 3', 3),
    ('Space Force', 'E-4', 'Spc4', 'Specialist 4', 4),
    ('Space Force', 'E-5', 'Sgt', 'Sergeant', 5),
    ('Space Force', 'E-6', 'TSgt', 'Technical Sergeant', 6),
    ('Space Force', 'E-7', 'MSgt', 'Master Sergeant', 7),
    ('Space Force', 'E-8', 'SMSgt', 'Senior Master Sergeant', 8),
    ('Space Force', 'E-9', 'CMSgt', 'Chief Master Sergeant', 9),
    ('Space Force', 'O-1', '2d Lt', 'Second Lieutenant', 15),
    ('Space Force', 'O-2', '1st Lt', 'First Lieutenant', 16),
    ('Space Force', 'O-3', 'Capt', 'Captain', 17),
    ('Space Force', 'O-4', 'Maj', 'Major', 18),
    ('Space Force', 'O-5', 'Lt Col', 'Lieutenant Colonel', 19),
    ('Space Force', 'O-6', 'Col', 'Colonel', 20),
    ('Space Force', 'O-7', 'Brig Gen', 'Brigadier General', 21),
    ('Space Force', 'O-8', 'Maj Gen', 'Major General', 22),
    ('Space Force', 'O-9', 'Lt Gen', 'Lieutenant General', 23),
    ('Space Force', 'O-10', 'Gen', 'General', 24),
    ('Coast Guard', 'E-1', 'SR', 'Seaman Recruit', 1),
    ('Coast Guard', 'E-2', 'SA', 'Seaman Apprentice', 2),
    ('Coast Guard', 'E-3', 'SN', 'Seaman', 3),
    ('Coast Guard', 'E-4', 'PO3', 'Petty Officer Third Class', 4),
    ('Coast Guard', 'E-5', 'PO2', 'Petty Officer Second Class', 5),
    ('Coast Guard', 'E-6', 'PO1', 'Petty Officer First Class', 6),
    ('Coast Guard', 'E-7', 'CPO', 'Chief Petty Officer', 7),
    ('Coast Guard', 'E-8', 'SCPO', 'Senior Chief Petty Officer', 8),
    ('Coast Guard', 'E-9', 'MCPO', 'Master Chief Petty Officer', 9),
    ('Coast Guard', 'W-2', 'CWO2', 'Chief Warrant Officer 2', 11),
    ('Coast Guard', 'W-3', 'CWO3', 'Chief Warrant Officer 3', 12),
    ('Coast Guard', 'W-4', 'CWO4', 'Chief Warrant Officer 4', 13),
    ('Coast Guard', 'O-1', 'ENS', 'Ensign', 15),
    ('Coast Guard', 'O-2', 'LTJG', 'Lieutenant Junior Grade', 16),
    ('Coast Guard', 'O-3', 'LT', 'Lieutenant', 17),
    ('Coast Guard', 'O-4', 'LCDR', 'Lieutenant Commander', 18),
    ('Coast Guard', 'O-5', 'CDR', 'Commander', 19),
    ('Coast Guard', 'O-6', 'CAPT', 'Captain', 20),
    ('Coast Guard', 'O-7', 'RDML', 'Rear Admiral Lower Half', 21),
    ('Coast Guard', 'O-8', 'RADM', 'Rear Admiral Upper Half', 22),
    ('Coast Guard', 'O-9', 'VADM', 'Vice Admiral', 23),
    ('Coast Guard', 'O-10', 'ADM', 'Admiral', 24)
ON CONFLICT (service_branch, pay_grade) DO UPDATE SET
    rank_abbreviation = EXCLUDED.rank_abbreviation,
    rank_title = EXCLUDED.rank_title,
    sort_order = EXCLUDED.sort_order;
