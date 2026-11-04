"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { AlertCircleIcon, Loader2Icon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSession } from "@/components/providers/session-provider";
import { loginSchema, registerSchema, type LoginValues } from "@/lib/validations";
import type { Resolver } from "react-hook-form";

/** One form model across both modes; confirmPassword exists only on register. */
type AuthFormValues = LoginValues & { confirmPassword?: string };

function roleHome(role: string | undefined): string {
  return role === "provider" ? "/provider" : "/dashboard";
}

interface AuthFormProps {
  mode: "login" | "register";
}

export function AuthForm({ mode }: AuthFormProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { refresh } = useSession();
  const [serverError, setServerError] = useState<string | null>(null);

  const isLogin = mode === "login";

  const form = useForm<AuthFormValues>({
    resolver: zodResolver(isLogin ? loginSchema : registerSchema) as Resolver<AuthFormValues>,
    defaultValues: { email: "", password: "", confirmPassword: "" },
  });

  const {
    register: registerField,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = form;

  async function onSubmit(values: AuthFormValues) {
    setServerError(null);
    try {
      const response = await fetch(`/api/auth/${mode}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(
          isLogin
            ? { email: values.email, password: values.password }
            : { email: values.email, password: values.password },
        ),
      });
      const data = (await response.json().catch(() => ({}))) as {
        user?: { role: string };
        error?: string;
      };

      if (!response.ok) {
        setServerError(data.error ?? "Something went wrong. Please try again.");
        return;
      }

      await refresh();

      // A pending slot from the landing page flows through to discovery.
      const slotParam = searchParams.get("slot");
      const nextParam = searchParams.get("next");
      if (slotParam) {
        router.replace(`/dashboard/discover?slot=${slotParam}`);
      } else if (
        nextParam &&
        nextParam.startsWith("/") &&
        !nextParam.startsWith("//") &&
        !nextParam.startsWith("/\\")
      ) {
        router.replace(nextParam);
      } else {
        router.replace(roleHome(data.user?.role));
      }
      router.refresh();
    } catch {
      setServerError("Cannot reach the server. Please try again.");
    }
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight">
        {isLogin ? "Welcome back" : "Create your account"}
      </h1>
      <p className="mt-1.5 text-sm text-muted-foreground">
        {isLogin
          ? "Sign in to manage your appointments."
          : "Book open slots the moment providers publish them."}
      </p>

      <form
        onSubmit={handleSubmit(onSubmit)}
        className="mt-8 space-y-4"
        noValidate
      >
        {serverError ? (
          <div
            role="alert"
            className="flex items-start gap-2.5 rounded-lg border border-destructive/30 bg-destructive/5 px-3.5 py-3 text-sm text-destructive"
          >
            <AlertCircleIcon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            {serverError}
          </div>
        ) : null}

        <div className="space-y-1.5">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            aria-invalid={Boolean(errors.email)}
            {...registerField("email")}
          />
          {errors.email ? (
            <p className="text-xs text-destructive">{errors.email.message}</p>
          ) : null}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete={isLogin ? "current-password" : "new-password"}
            placeholder={isLogin ? "Your password" : "At least 12 characters"}
            aria-invalid={Boolean(errors.password)}
            {...registerField("password")}
          />
          {errors.password ? (
            <p className="text-xs text-destructive">{errors.password.message}</p>
          ) : null}
        </div>

        {!isLogin ? (
          <div className="space-y-1.5">
            <Label htmlFor="confirmPassword">Confirm password</Label>
            <Input
              id="confirmPassword"
              type="password"
              autoComplete="new-password"
              aria-invalid={Boolean(errors.confirmPassword)}
              {...registerField("confirmPassword")}
            />
            {errors.confirmPassword ? (
              <p className="text-xs text-destructive">
                {errors.confirmPassword.message}
              </p>
            ) : null}
          </div>
        ) : null}

        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {isSubmitting ? (
            <Loader2Icon className="animate-spin" aria-hidden="true" />
          ) : null}
          {isLogin ? "Sign in" : "Create account"}
        </Button>
      </form>

      <p className="mt-6 text-center text-sm text-muted-foreground">
        {isLogin ? (
          <>
            New to SlotForge?{" "}
            <Link href="/register" className="font-medium text-foreground underline-offset-4 hover:underline">
              Create an account
            </Link>
          </>
        ) : (
          <>
            Already have an account?{" "}
            <Link href="/login" className="font-medium text-foreground underline-offset-4 hover:underline">
              Sign in
            </Link>
          </>
        )}
      </p>

      {!isLogin ? (
        <p className="mt-4 text-center text-xs text-muted-foreground/80">
          Accounts are created with the customer role. Provider accounts are
          provisioned by the SlotForge team.
        </p>
      ) : null}
    </div>
  );
}
