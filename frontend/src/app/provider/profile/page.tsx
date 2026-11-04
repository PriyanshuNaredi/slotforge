"use client";

import { useState } from "react";
import { AlertCircleIcon, Loader2Icon, UserRoundIcon } from "lucide-react";
import { toast } from "sonner";

import { createProviderProfile, getMyProviderProfile } from "@/lib/api/endpoints";
import type { ProviderProfile } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { profileSchema } from "@/lib/validations";
import { formatRelative, timeZoneAbbr } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useAsyncData } from "@/hooks/use-async-data";

function viewerTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return "UTC";
  }
}

const TIMEZONES = (() => {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return ["UTC", viewerTimezone()];
  }
})();

export default function ProviderProfilePage() {
  const profile = useAsyncData("my-profile", () => getMyProviderProfile());
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [timezone, setTimezone] = useState(viewerTimezone());

  async function submit() {
    setError(null);
    const parsed = profileSchema.safeParse({ display_name: displayName, bio, timezone });
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message ?? "Invalid profile details");
      return;
    }

    setSaving(true);
    try {
      await createProviderProfile({
        display_name: parsed.data.display_name,
        bio: parsed.data.bio ? parsed.data.bio : undefined,
        timezone: parsed.data.timezone,
      });
      toast.success("Profile created", {
        description: "Customers now see your name on every open slot.",
      });
      profile.reload();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError("You already have a profile.");
        profile.reload();
      } else {
        setError(toErrorMessage(err));
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <PageHeader
        title="Provider profile"
        description="How customers see you across SlotForge."
      />

      {profile.error ? (
        <ErrorState
          icon={UserRoundIcon}
          title="Couldn't load your profile"
          description={profile.error}
          action={
            <Button variant="outline" size="sm" onClick={profile.reload}>
              Retry
            </Button>
          }
        />
      ) : profile.loading ? (
        <Skeleton className="h-72 w-full rounded-xl" />
      ) : profile.data ? (
        <ExistingProfile profile={profile.data} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Create your profile</CardTitle>
            <CardDescription>
              Your display name and timezone appear on every slot you publish.
              Profiles are created once — pick your timezone carefully.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {error ? (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive"
              >
                <AlertCircleIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                {error}
              </div>
            ) : null}

            <div className="grid gap-1.5">
              <Label htmlFor="display-name">Display name</Label>
              <Input
                id="display-name"
                placeholder="Dr. Amara Marsh"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                aria-invalid={Boolean(error)}
                maxLength={150}
              />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="bio">
                Bio <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Textarea
                id="bio"
                rows={4}
                placeholder="What you offer, specialties, what customers should expect…"
                value={bio}
                onChange={(event) => setBio(event.target.value)}
                maxLength={2000}
              />
            </div>

            <div className="grid gap-1.5">
              <Label htmlFor="timezone">Timezone</Label>
              <Select value={timezone} onValueChange={setTimezone}>
                <SelectTrigger id="timezone" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="max-h-72">
                  {TIMEZONES.map((tz) => (
                    <SelectItem key={tz} value={tz}>
                      {tz} ({timeZoneAbbr(tz)})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Button variant="brand" onClick={() => void submit()} disabled={saving}>
              {saving ? <Loader2Icon className="animate-spin" aria-hidden="true" /> : null}
              Create profile
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function ExistingProfile({ profile }: { profile: ProviderProfile }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-3">
          <span className="flex size-10 items-center justify-center rounded-full bg-brand-soft text-sm font-semibold text-brand-strong">
            {profile.display_name.slice(0, 2).toUpperCase()}
          </span>
          {profile.display_name}
        </CardTitle>
        <CardDescription>
          Created {formatRelative(new Date(profile.created_at))} ·{" "}
          {profile.timezone} ({timeZoneAbbr(profile.timezone)})
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <p className="text-sm font-medium">Bio</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {profile.bio?.trim() ? profile.bio : "No bio added."}
          </p>
        </div>
        <p className="rounded-lg border border-dashed bg-muted/40 px-4 py-3 text-xs text-muted-foreground">
          Profile editing isn&apos;t available yet — the backend currently
          supports creating a profile once. Changes will be possible when an
          update endpoint ships.
        </p>
      </CardContent>
    </Card>
  );
}
