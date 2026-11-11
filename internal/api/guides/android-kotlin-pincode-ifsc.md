---
title: Pincode autofill and IFSC lookup in an Android app (Kotlin)
description: Fill in district and state from a pincode and check an IFSC code in an Android app with Kotlin coroutines and HttpURLConnection, no library and no key.
apis: pincode, ifsc
published: 2026-11-11
---

An Android form that takes an Indian address or bank account can fill itself in: a pincode gives the district and state, an IFSC code gives the bank and branch. Both are one GET request with no key, and the code below needs no library beyond Kotlin coroutines.

## Pincode to district and state

```kotlin
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

data class Place(val district: String, val state: String, val offices: List<String>)

suspend fun lookUpPincode(pincode: String): Place? = withContext(Dispatchers.IO) {
    if (!Regex("^[1-9]\\d{5}$").matches(pincode)) return@withContext null
    val connection = URL("https://api.dharun.dev/v1/pincode/${pincode}?utm_medium=guide").openConnection() as HttpURLConnection
    try {
        if (connection.responseCode != 200) return@withContext null
        val data = JSONObject(connection.inputStream.bufferedReader().readText()).getJSONObject("data")
        val offices = data.getJSONArray("offices")
        Place(data.getString("district"), data.getString("state"),
            List(offices.length()) { offices.getJSONObject(it).getString("name") })
    } finally {
        connection.disconnect()
    }
}
```

Call it from a `lifecycleScope.launch` when the field reaches six digits. The answer for 637001 has the district Namakkal, the state Tamil Nadu and its post offices: https://api.dharun.dev/v1/pincode/637001?utm_medium=guide

The app needs `<uses-permission android:name="android.permission.INTERNET" />` in its manifest.

## IFSC to bank and branch

```kotlin
suspend fun describeIfsc(code: String): String = withContext(Dispatchers.IO) {
    val ifsc = code.trim().uppercase()
    if (!Regex("^[A-Z]{4}0[A-Z0-9]{6}$").matches(ifsc)) return@withContext "An IFSC is 11 characters, the fifth a zero."
    val connection = URL("https://api.dharun.dev/v1/ifsc/${ifsc}?utm_medium=guide").openConnection() as HttpURLConnection
    try {
        if (connection.responseCode == 404) return@withContext "No branch has this IFSC."
        val data = JSONObject(connection.inputStream.bufferedReader().readText()).getJSONObject("data")
        "${data.getJSONObject("bank").getString("name")}, ${data.getString("branch")}, ${data.getString("city")}"
    } finally {
        connection.disconnect()
    }
}
```

The answer also gives the MICR code and whether the branch takes NEFT, RTGS, IMPS and UPI: https://api.dharun.dev/v1/ifsc/SBIN0000001?utm_medium=guide

## Limits

No key is needed. A free key from https://api.dharun.dev/signup raises the per-minute limit; send it as `Authorization: Bearer <key>`. Keep keys out of a published app's code: put the lookups behind your own server if you use one.
