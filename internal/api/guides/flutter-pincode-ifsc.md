---
title: Pincode autofill and IFSC lookup in a Flutter app
description: Fill in district and state from a pincode, check an IFSC code and show a company logo in Flutter, with the http package and no API key.
apis: pincode, ifsc, logos
published: 2026-11-11
---

Address and bank forms in Indian apps ask for the same things: a pincode that should fill in the district and state, and an IFSC code that should show the bank and branch. Both are one GET request, with no key.

## Add the http package

```sh
flutter pub add http
```

## Pincode to district and state

```dart
import 'dart:convert';
import 'package:http/http.dart' as http;

class Place {
  final String district, state;
  final List<String> offices;
  Place(this.district, this.state, this.offices);
}

Future<Place?> lookUpPincode(String pincode) async {
  if (!RegExp(r'^[1-9]\d{5}$').hasMatch(pincode)) return null;
  final response = await http.get(Uri.parse('https://api.dharun.dev/v1/pincode/${pincode}?utm_medium=guide'));
  if (response.statusCode != 200) return null;
  final data = jsonDecode(response.body)['data'];
  return Place(data['district'], data['state'],
      [for (final office in data['offices']) office['name'] as String]);
}
```

Call it when the field reaches six digits, and fill the district and state fields from the answer. A pincode often has several post offices; show them as choices for the locality.

The answer for 637001:

```json
{ "data": { "pincode": "637001", "district": "Namakkal", "state": "Tamil Nadu",
  "offices": [ { "name": "Namakkal H.O", "type": "Head Office", "delivery": true } ] } }
```

Try it: https://api.dharun.dev/v1/pincode/637001?utm_medium=guide

## IFSC to bank and branch

```dart
Future<String?> describeIfsc(String code) async {
  final ifsc = code.trim().toUpperCase();
  if (!RegExp(r'^[A-Z]{4}0[A-Z0-9]{6}$').hasMatch(ifsc)) return 'An IFSC is 11 characters, the fifth a zero.';
  final response = await http.get(Uri.parse('https://api.dharun.dev/v1/ifsc/${ifsc}?utm_medium=guide'));
  if (response.statusCode == 404) return 'No branch has this IFSC.';
  final data = jsonDecode(response.body)['data'];
  return '${data['bank']['name']}, ${data['branch']}, ${data['city']}';
}
```

The answer also says whether the branch takes NEFT, RTGS, IMPS and UPI, and gives its MICR code: https://api.dharun.dev/v1/ifsc/SBIN0000001?utm_medium=guide

## A company's logo in a list

Logos are plain image addresses, so `Image.network` is enough:

```dart
Image.network('https://api.dharun.dev/v1/logos/TCS?format=png&size=64&utm_medium=guide', width: 32, height: 32)
```

An unknown name gets a lettered placeholder instead of an error, so a list never shows a broken image: https://api.dharun.dev/v1/logos/TCS?format=png&size=64&utm_medium=guide

## Limits

No key is needed. Without one, an address may make a few hundred requests a minute; a free key from https://api.dharun.dev/signup raises that. Cache the answers you reuse: pincodes and IFSC codes change rarely.
