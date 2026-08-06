import urllib.request
import json
import urllib.error

url = 'http://localhost:8081/api/v1/driver/start-free-trip'
data = json.dumps({"id_driver": 3, "id_kendaraan": 2}).encode('utf-8')
req = urllib.request.Request(url, data=data, headers={'Content-Type': 'application/json'})

try:
    with urllib.request.urlopen(req) as response:
        print("Status:", response.status)
        print("Body:", response.read().decode('utf-8'))
except urllib.error.HTTPError as e:
    print("HTTP Error:", e.code)
    print("Body:", e.read().decode('utf-8'))
except Exception as e:
    print("Error:", e)
