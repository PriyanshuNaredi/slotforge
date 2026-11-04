param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$PostgresContainer = "slotforge-postgres",
    [string]$Database = "slotforge",
    [string]$DatabaseUser = "slotforge"
)

$ErrorActionPreference = "Stop"

# Creates isolated test records, exercises the real API, verifies the notification,
# then removes only the records created by this run.
$suffix = [guid]::NewGuid().ToString("N")
$customerId = [guid]::NewGuid().ToString()
$providerId = [guid]::NewGuid().ToString()
$slotId = [guid]::NewGuid().ToString()

$customerEmail = "slotforge-test-customer-$suffix@example.test"
$providerEmail = "slotforge-test-provider-$suffix@example.test"
$testPassword = "SfTest-$suffix!"

$bookingId = $null
$customerToken = $null
$providerToken = $null
$hashHelper = Join-Path (Get-Location) "slotforge_hashgen_$suffix.go"
$dbSetupAttempted = $false

function Invoke-DatabaseSql {
    param([Parameter(Mandatory = $true)][string]$Sql)

    $output = & docker exec $PostgresContainer psql `
        -U $DatabaseUser -d $Database -v ON_ERROR_STOP=1 -t -A -c $Sql

    if ($LASTEXITCODE -ne 0) {
        throw "PostgreSQL command failed."
    }

    return (($output | Out-String).Trim())
}

function New-PasswordHash {
    param(
        [string]$Password,
        [string]$HelperPath
    )

    $goSource = @'
package main

import (
    "fmt"
    "os"

    "golang.org/x/crypto/bcrypt"
)

func main() {
    if len(os.Args) != 2 {
        panic("expected password argument")
    }
    hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
    if err != nil {
        panic(err)
    }
    fmt.Print(string(hash))
}
'@

    Set-Content -LiteralPath $HelperPath -Value $goSource -Encoding UTF8
    $result = & go run $HelperPath $Password

    if ($LASTEXITCODE -ne 0) {
        throw "Could not generate bcrypt hash. Check Go and golang.org/x/crypto."
    }

    return (($result | Out-String).Trim())
}

try {
    Write-Host "`n=== SlotForge provider notification integration test ===" -ForegroundColor Cyan

    Write-Host "`n[1/6] Checking API and PostgreSQL..."
    $null = Invoke-RestMethod -Method Get -Uri "$BaseUrl/healthz"
    $null = Invoke-DatabaseSql -Sql "SELECT 1;"
    Write-Host "PASS: API and PostgreSQL are reachable." -ForegroundColor Green

    Write-Host "`n[2/6] Creating temporary customer, provider, profile, and slot..."
    $passwordHash = New-PasswordHash -Password $testPassword -HelperPath $hashHelper

    $setupSql = @"
INSERT INTO users (id, email, password_hash, role)
VALUES
    ('$customerId', '$customerEmail', '$passwordHash', 'customer'),
    ('$providerId', '$providerEmail', '$passwordHash', 'provider');

INSERT INTO provider_profiles (user_id, display_name, bio, timezone)
VALUES ('$providerId', 'SlotForge Automated Test Provider', 'Temporary integration-test profile', 'UTC');

INSERT INTO availability_slots (id, provider_id, starts_at, ends_at, status)
VALUES (
    '$slotId',
    '$providerId',
    NOW() + INTERVAL '48 hours',
    NOW() + INTERVAL '48 hours 30 minutes',
    'open'
);
"@

    $dbSetupAttempted = $true
    $null = Invoke-DatabaseSql -Sql $setupSql
    Write-Host "PASS: Temporary test records created." -ForegroundColor Green

    Write-Host "`n[3/6] Logging in as customer and creating booking..."
    $customerLoginBody = @{
        email = $customerEmail
        password = $testPassword
    } | ConvertTo-Json

    $customerAuth = Invoke-RestMethod `
        -Method Post -Uri "$BaseUrl/api/v1/auth/login" `
        -ContentType "application/json" -Body $customerLoginBody

    $customerToken = $customerAuth.access_token
    if (-not $customerToken) { throw "Customer login returned no access token." }

    $bookingBody = @{ slot_id = $slotId } | ConvertTo-Json
    $booking = Invoke-RestMethod `
        -Method Post -Uri "$BaseUrl/api/v1/bookings" `
        -Headers @{ Authorization = "Bearer $customerToken" } `
        -ContentType "application/json" -Body $bookingBody

    $bookingId = [string]$booking.id
    if ([string]::IsNullOrWhiteSpace($bookingId)) {
        throw "Booking API response did not contain an id."
    }
    Write-Host "PASS: Booking created: $bookingId" -ForegroundColor Green

    Write-Host "`n[4/6] Logging in as provider and completing booking..."
    $providerLoginBody = @{
        email = $providerEmail
        password = $testPassword
    } | ConvertTo-Json

    $providerAuth = Invoke-RestMethod `
        -Method Post -Uri "$BaseUrl/api/v1/auth/login" `
        -ContentType "application/json" -Body $providerLoginBody

    $providerToken = $providerAuth.access_token
    if (-not $providerToken) { throw "Provider login returned no access token." }
    if ($providerAuth.user.role -ne "provider") {
        throw "Temporary account did not receive provider role."
    }

    $statusBody = @{ status = "completed" } | ConvertTo-Json
    $null = Invoke-RestMethod `
        -Method Patch -Uri "$BaseUrl/api/v1/providers/bookings/$bookingId/status" `
        -Headers @{ Authorization = "Bearer $providerToken" } `
        -ContentType "application/json" -Body $statusBody

    Write-Host "PASS: Provider changed booking status to completed." -ForegroundColor Green

    Write-Host "`n[5/6] Verifying booking status and customer notification..."
    $statusCount = Invoke-DatabaseSql -Sql @"
SELECT COUNT(*) FROM bookings
WHERE id = '$bookingId' AND status = 'completed' AND customer_id = '$customerId';
"@
    if ([int]$statusCount -ne 1) {
        throw "Expected one completed booking; found $statusCount."
    }

    $notificationCount = Invoke-DatabaseSql -Sql @"
SELECT COUNT(*) FROM notifications
WHERE booking_id = '$bookingId'
  AND user_id = '$customerId'
  AND type = 'booking_completed';
"@
    if ([int]$notificationCount -ne 1) {
        throw "Expected one booking_completed customer notification; found $notificationCount."
    }

    Write-Host "PASS: Completed booking and customer notification verified." -ForegroundColor Green
    Write-Host "`nALL TEST ASSERTIONS PASSED." -ForegroundColor Green
}
catch {
    Write-Host "`nTEST FAILED: $($_.Exception.Message)" -ForegroundColor Red
    throw
}
finally {
    if ($dbSetupAttempted) {
        Write-Host "`n[6/6] Cleaning up temporary test data..."

        $bookingFilter = ""
        if ($bookingId) {
            $bookingFilter = " OR booking_id = '$bookingId'"
        }

        $cleanupSql = @"
DELETE FROM notifications
WHERE user_id IN ('$customerId', '$providerId')$bookingFilter;

DELETE FROM bookings
WHERE slot_id = '$slotId' OR customer_id = '$customerId';

DELETE FROM availability_slots WHERE id = '$slotId';
DELETE FROM provider_profiles WHERE user_id = '$providerId';
DELETE FROM users WHERE id IN ('$customerId', '$providerId');
"@

        try {
            $null = Invoke-DatabaseSql -Sql $cleanupSql
            Write-Host "PASS: Temporary users and related records removed." -ForegroundColor Green
        }
        catch {
            Write-Host "CLEANUP WARNING: $($_.Exception.Message)" -ForegroundColor Yellow
            Write-Host "Temporary IDs: customer=$customerId provider=$providerId slot=$slotId booking=$bookingId" -ForegroundColor Yellow
        }
    }

    if (Test-Path -LiteralPath $hashHelper) {
        Remove-Item -LiteralPath $hashHelper -Force
    }

    $customerToken = $null
    $providerToken = $null
    $testPassword = $null
}

