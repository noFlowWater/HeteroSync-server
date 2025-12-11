# HeteroSync Server User Guide

이 가이드는 HeteroSync Server의 고급 사용법과 실제 시나리오를 다룹니다.

---

## 목차

1. [NTP 다중 샘플링 사용](#ntp-다중-샘플링-사용)
2. [Auto-Sync 활용](#auto-sync-활용)
3. [페어링 자동 복구](#페어링-자동-복구)
4. [EDF 후처리 시나리오](#edf-후처리-시나리오)
5. [연결 상태 모니터링](#연결-상태-모니터링)
6. [트러블슈팅](#트러블슈팅)

---

## NTP 다중 샘플링 사용

### EDF 측정 시작 전 동기화

수면 측정 시작 전에 정밀한 시간 오프셋을 측정하세요:

```bash
# EDF 측정 시작 직전
curl -X POST http://localhost:8080/api/sync/multi \
  -H "Content-Type: application/json" \
  -d '{
    "pairing_id": "psg-watch-pair",
    "sample_count": 10,
    "interval_ms": 200,
    "timeout_sec": 5
  }'
```

**응답 예시**:
```json
{
  "success": true,
  "result": {
    "aggregation_id": "agg-uuid-xxx",
    "best_offset": -150,
    "confidence": 0.94,
    "jitter": 2000.0,
    "total_samples": 10,
    "valid_samples": 8
  }
}
```

**해석**:
- `best_offset: -150ms` → PSG 클럭이 Watch보다 150ms 느림
- `confidence: 0.94` → 매우 신뢰할 수 있는 측정
- `jitter: 2000.0μs` → 네트워크 안정적 (2ms 변동)

---

### EDF 측정 종료 후 동기화

8시간 수면 측정 후 다시 동기화하여 클럭 드리프트를 측정:

```bash
# EDF 측정 종료 직후 (8시간 후)
curl -X POST http://localhost:8080/api/sync/multi \
  -H "Content-Type: application/json" \
  -d '{
    "pairing_id": "psg-watch-pair",
    "sample_count": 10
  }'
```

**응답 예시**:
```json
{
  "success": true,
  "result": {
    "best_offset": -182,
    "confidence": 0.92
  }
}
```

**클럭 드리프트 계산**:
```
시작 오프셋: -150ms
종료 오프셋: -182ms
드리프트: -182 - (-150) = -32ms (8시간 동안)
드리프트율: -32ms / (8 * 3600 * 1000ms) = -1.1 ppm
```

---

## Auto-Sync 활용

### 시나리오: 장기간 연속 측정

24시간 연속 측정 중 자동으로 시간 동기화를 수행:

```bash
# 1. 페어링 생성 시 Auto-Sync 설정
curl -X POST http://localhost:8080/api/pairings \
  -H "Content-Type: application/json" \
  -d '{
    "device1Id": "psg-001",
    "device2Id": "watch-001",
    "autoSyncIntervalSec": 3600,
    "autoSyncSampleCount": 15,
    "autoSyncIntervalMs": 200
  }'

# 응답: {"pairingId": "pair-123"}
```

**결과**:
- 1시간마다 자동으로 NTP 다중 샘플링 수행
- 각 동기화마다 15개 샘플 측정
- 결과가 자동으로 DB에 저장됨

---

### Auto-Sync 상태 모니터링

```bash
# 실시간 상태 확인
curl http://localhost:8080/api/auto-sync/status?pairingId=pair-123 | jq

# 출력:
{
  "pairing_id": "pair-123",
  "status": "RUNNING",
  "config": {
    "interval_sec": 3600,
    "sample_count": 15
  },
  "last_sync_at": "2025-10-28T10:05:00Z",
  "last_sync_success": true,
  "total_syncs": 24,
  "failed_syncs": 0
}
```

---

### Auto-Sync 데이터 조회

```bash
# 24시간 동안 수집된 집계 결과 조회
curl "http://localhost:8080/api/sync/aggregated?pairingId=pair-123&limit=100" | jq

# 시간에 따른 오프셋 변화 분석
curl "http://localhost:8080/api/sync/aggregated?pairingId=pair-123" | \
  jq '.[] | {time: .created_at, offset: .best_offset, confidence: .confidence}'
```

**결과 활용**:
- 시간에 따른 클럭 드리프트 추적
- 선형 보간으로 정밀한 타임스탬프 보정
- 신뢰도가 낮은 구간 식별

---

## 페어링 자동 복구

### 시나리오 1: 디바이스 재연결 자동 복구

장기간 수면 측정 중 네트워크 연결이 일시적으로 끊어졌다가 재연결되는 경우:

```bash
# 1. 초기 페어링 생성 (DB에 저장됨)
curl -X POST http://localhost:8080/api/pairings \
  -H "Content-Type: application/json" \
  -d '{
    "device1Id": "psg-001",
    "device2Id": "watch-001",
    "autoSyncIntervalSec": 600,
    "autoSyncSampleCount": 15
  }'

# 응답: {"pairingId": "abc-123"}
# 서버 로그: "Auto-sync automatically started for pairing abc-123"
```

**디바이스 연결 해제 (예: 네트워크 끊김)**:
- in-memory 페어링 삭제
- Auto-Sync 중단
- ✅ DB의 페어링은 그대로 유지

**디바이스 재연결 (WebSocket 재연결)**:

서버가 자동으로 감지하여:
1. DB에서 페어링 조회
2. 상대 디바이스 연결 확인
3. 페어링 자동 복구 (in-memory)
4. Auto-Sync 자동 재시작

**서버 로그**:
```
Client registered: watch-001 (WATCH)
Found 1 pairing(s) for device watch-001, checking for restoration
✓ Pairing restored: abc-123 (psg-001 <-> watch-001)
✓ Auto-Sync automatically restarted for pairing abc-123 (interval: 600s, samples: 15)
```

**상태 확인**:
```bash
curl http://localhost:8080/api/auto-sync/status?pairingId=abc-123 | jq

# 출력: Auto-Sync이 정상 실행 중임을 확인
```

**결과**:
- 네트워크 재연결 후 **수동 개입 없이** 자동으로 데이터 수집 재개
- Continuous 데이터 수집 보장
- 장시간 측정 시나리오에 최적화

---

### 시나리오 2: 서버 재시작 후 복구

서버를 재시작하더라도 페어링 정보가 DB에 저장되어 있어 복구 가능:

```bash
# 1. 서버 종료
# Ctrl+C 또는 kill 명령어

# 2. 서버 재시작
./server

# 3. 디바이스들이 자동 재연결되면
# - Pairing Operator가 DB에서 페어링 조회
# - 자동으로 페어링 및 Auto-Sync 복구

# 4. 페어링 목록 확인
curl http://localhost:8080/api/pairings | jq

# 응답: DB에 저장된 모든 페어링 조회 가능
```

---

## EDF 후처리 시나리오

### 시간축 정렬을 위한 오프셋 사용

EDF 파일과 웨어러블 데이터의 타임스탬프를 정렬하는 예시:

```python
# Python 예시: EDF와 Watch 데이터 시간축 정렬
import requests

# 1. 측정 시작 시점의 오프셋
start_response = requests.post(
    'http://localhost:8080/api/sync/multi',
    json={'pairing_id': 'psg-watch-pair', 'sample_count': 10}
)
start_offset = start_response.json()['result']['best_offset']  # -150ms

# 2. 측정 종료 시점의 오프셋 (8시간 후)
end_response = requests.post(
    'http://localhost:8080/api/sync/multi',
    json={'pairing_id': 'psg-watch-pair', 'sample_count': 10}
)
end_offset = end_response.json()['result']['best_offset']  # -182ms

# 3. EDF 데이터와 Watch 데이터 시간축 정렬
edf_start_time = 1727870400000  # EDF 시작 시간 (ms)
edf_end_time = edf_start_time + (8 * 3600 * 1000)  # 8시간 후
duration = edf_end_time - edf_start_time

# 선형 보간으로 시간 보정
for edf_timestamp in edf_data:
    elapsed = edf_timestamp - edf_start_time
    progress = elapsed / duration

    # 시작과 종료 오프셋 사이를 선형 보간
    interpolated_offset = start_offset + progress * (end_offset - start_offset)

    # Watch 데이터와 정렬된 타임스탬프
    aligned_timestamp = edf_timestamp + interpolated_offset

    # 정렬된 타임스탬프로 Watch 데이터 조회
    watch_data = get_watch_data_at(aligned_timestamp)
```

---

### Auto-Sync 데이터를 이용한 정밀 보정

Auto-Sync로 주기적으로 수집된 데이터를 사용하면 더 정밀한 보정 가능:

```python
import requests
import numpy as np
from scipy.interpolate import interp1d

# 1. Auto-Sync로 수집된 모든 오프셋 조회
response = requests.get(
    'http://localhost:8080/api/sync/aggregated',
    params={'pairingId': 'psg-watch-pair', 'limit': 1000}
)
sync_results = response.json()

# 2. 시간과 오프셋 배열 생성
times = [r['created_at'] for r in sync_results]
offsets = [r['best_offset'] for r in sync_results]

# 3. 보간 함수 생성 (선형 또는 스플라인)
offset_func = interp1d(times, offsets, kind='linear', fill_value='extrapolate')

# 4. EDF 데이터의 각 타임스탬프에 대한 오프셋 계산
for edf_timestamp in edf_data:
    interpolated_offset = offset_func(edf_timestamp)
    aligned_timestamp = edf_timestamp + interpolated_offset

    # 정렬된 타임스탬프 사용
    watch_data = get_watch_data_at(aligned_timestamp)
```

---

## 연결 상태 모니터링

### 실시간 연결 건강도 확인

```bash
# 모든 디바이스의 건강도 조회
curl http://localhost:8080/api/devices/health | jq

# 출력:
[
  {
    "deviceId": "psg-001",
    "deviceType": "PSG",
    "lastRtt": 15,
    "isHealthy": true,
    "timeSinceLastPong": 5000
  },
  {
    "deviceId": "watch-001",
    "deviceType": "WATCH",
    "lastRtt": 25,
    "isHealthy": true,
    "timeSinceLastPong": 7000
  }
]
```

---

### 모니터링 대시보드 스크립트

```bash
# 주기적으로 연결 상태 확인
watch -n 5 'curl -s http://localhost:8080/api/devices/health | jq'

# 비건강 디바이스 필터링 (jq 사용)
curl http://localhost:8080/api/devices/health | jq '.[] | select(.isHealthy == false)'

# 특정 디바이스가 건강한지 확인
curl "http://localhost:8080/api/devices/health?deviceId=psg-001" | jq '.isHealthy'
# 출력: true
```

---

### 연결 상태 알림 스크립트

```bash
#!/bin/bash
# check_health.sh - 연결 상태 모니터링 및 알림

DEVICE_ID="psg-001"
API_URL="http://localhost:8080/api/devices/health?deviceId=$DEVICE_ID"

while true; do
  HEALTH=$(curl -s "$API_URL" | jq -r '.isHealthy')

  if [ "$HEALTH" != "true" ]; then
    echo "⚠️ WARNING: Device $DEVICE_ID is unhealthy!"
    # 알림 전송 (예: Slack, Email)
    # notify_slack "Device $DEVICE_ID connection issue"
  fi

  sleep 30
done
```

---

## 트러블슈팅

### 문제 1: 디바이스 연결이 자주 끊김

**증상**:
- WebSocket 연결이 90초 이상 유지되지 않음
- `isHealthy: false` 상태 반복

**해결 방법**:

1. **클라이언트 PONG 응답 확인**:
   ```javascript
   websocket.onmessage = (event) => {
     const message = JSON.parse(event.data);
     if (message.type === 'PING') {
       // ⚠️ 즉시 PONG 응답 필요
       websocket.send(JSON.stringify({
         type: 'PONG',
         timestamp: Date.now()
       }));
     }
   };
   ```

2. **네트워크 방화벽 확인**:
   - WebSocket 포트(8080)가 방화벽에서 허용되는지 확인
   - NAT 타임아웃 설정 확인 (54초 프로토콜 레벨 PING이 이를 방지)

3. **클라이언트 로그 확인**:
   ```javascript
   websocket.onclose = (event) => {
     console.log('Connection closed:', event.code, event.reason);
     // code 1006: Abnormal closure (120초 타임아웃)
   };
   ```

---

### 문제 2: Auto-Sync가 시작되지 않음

**증상**:
- 페어링 생성 후 Auto-Sync 상태가 없음

**해결 방법**:

1. **페어링 상태 확인**:
   ```bash
   curl http://localhost:8080/api/pairings | jq
   ```

2. **디바이스 연결 상태 확인**:
   ```bash
   curl http://localhost:8080/api/devices | jq
   ```

3. **수동으로 Auto-Sync 시작**:
   ```bash
   curl -X POST http://localhost:8080/api/auto-sync/start \
     -H "Content-Type: application/json" \
     -d '{
       "pairing_id": "your-pairing-id",
       "interval_sec": 600,
       "sample_count": 15
     }'
   ```

4. **서버 로그 확인**:
   ```bash
   # Auto-Sync 관련 로그
   tail -f server.log | grep "auto-sync"
   ```

---

### 문제 3: NTP 동기화 confidence가 낮음

**증상**:
- `confidence < 0.7`
- `jitter > 10000μs` (10ms)

**해결 방법**:

1. **샘플 수 증가**:
   ```bash
   curl -X POST http://localhost:8080/api/sync/multi \
     -H "Content-Type: application/json" \
     -d '{
       "pairing_id": "pair-123",
       "sample_count": 20,
       "interval_ms": 500
     }'
   ```

2. **네트워크 안정성 확인**:
   ```bash
   # 디바이스 RTT 확인
   curl http://localhost:8080/api/devices/health | jq '.[] | {deviceId, lastRtt}'
   ```

3. **재측정**:
   - 네트워크 부하가 낮은 시간대에 재측정
   - WiFi 대신 유선 연결 사용

---

### 문제 4: 페어링 자동 복구가 안됨

**증상**:
- 디바이스 재연결 후 페어링이 복구되지 않음

**해결 방법**:

1. **DB에 페어링이 저장되어 있는지 확인**:
   ```bash
   curl http://localhost:8080/api/pairings | jq
   ```

2. **두 디바이스가 모두 연결되어 있는지 확인**:
   ```bash
   curl http://localhost:8080/api/devices | jq
   ```

3. **서버 로그 확인**:
   ```bash
   tail -f server.log | grep -E "restored|Pairing Operator"
   ```

4. **수동으로 페어링 재생성**:
   ```bash
   curl -X POST http://localhost:8080/api/pairings \
     -H "Content-Type: application/json" \
     -d '{
       "device1Id": "psg-001",
       "device2Id": "watch-001"
     }'
   ```

---

## 베스트 프랙티스

### 1. 측정 전 체크리스트

- [ ] 디바이스 WebSocket 연결 확인
- [ ] 디바이스 건강도 확인 (`isHealthy: true`)
- [ ] 페어링 생성 및 Auto-Sync 설정
- [ ] 초기 NTP 다중 샘플링 수행 (`confidence > 0.9`)

### 2. 측정 중 모니터링

- [ ] 주기적으로 디바이스 건강도 확인 (5분마다)
- [ ] Auto-Sync 상태 확인 (실패 여부)
- [ ] 서버 로그 모니터링

### 3. 측정 후 데이터 검증

- [ ] 종료 시점 NTP 다중 샘플링 수행
- [ ] 클럭 드리프트 계산
- [ ] 집계 결과 조회 및 신뢰도 확인
- [ ] 이상값 확인 (outlier_count)

---

## 추가 리소스

- [API 문서](../API.md) - 모든 REST API 엔드포인트
- [아키텍처 문서](ARCHITECTURE.md) - NTP 알고리즘 상세
- [WebSocket 프로토콜](WEBSOCKET.md) - 프로토콜 명세 및 클라이언트 구현
- [데이터베이스 스키마](DATABASE.md) - 테이블 구조 및 쿼리 예시
