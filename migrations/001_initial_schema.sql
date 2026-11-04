
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
