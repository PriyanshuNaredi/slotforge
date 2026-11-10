-- Provider unavailable periods: windows the provider blocks off.
-- Bookings may not be created or rescheduled into a blocked window, and new
-- availability may not overlap one. Existing bookings are never touched.
CREATE TABLE IF NOT EXISTS public.provider_unavailable_periods (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    provider_id uuid NOT NULL,
    starts_at timestamp with time zone NOT NULL,
    ends_at timestamp with time zone NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT provider_unavailable_periods_pkey PRIMARY KEY (id),
    CONSTRAINT unavailable_period_valid_time CHECK (ends_at > starts_at),
    CONSTRAINT provider_unavailable_periods_provider_fkey
        FOREIGN KEY (provider_id) REFERENCES public.users(id) ON DELETE CASCADE
);

-- GIST index so overlap checks (booking, availability creation) are index-backed.
CREATE INDEX IF NOT EXISTS unavailable_provider_range_idx
    ON public.provider_unavailable_periods
    USING gist (provider_id, tstzrange(starts_at, ends_at, '[)'::text));
