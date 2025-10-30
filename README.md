# Time Sync Server

웨어러블 디바이스(갤럭시 워치)와 PSG 장비(Windows PC) 간의 시간 동기화를 위한 WebSocket 기반 서버입니다.

## 기능

- **WebSocket 연결 관리**: PSG PC와 갤럭시 워치가 WebSocket으로 서버에 연결
- **PING/PONG 연결 모니터링**: 자동 연결 상태 확인 및 죽은 연결 감지
- **디바이스 페어링**: 연결된 디바이스들을 페어링하여 시간 동기화 준비
- **페어링 영구 저장 및 자동 복구**: 페어링 정보를 DB에 저장하여 디바이스 재연결 시 자동 복구 
- **자동 동기화 모니터링**: 페어링 생성 시 자동으로 주기적 동기화 시작 (백그라운드 실행)
- **시간 동기화**: 페어링된 두 디바이스에게 현재 시스템 시간을 요청하고 기록
- **NTP 다중 샘플링**: NTP 알고리즘 기반 정밀 시간 동기화 (8-10회 측정 후 최적값 선택)
- **동기화 이력 조회**: 저장된 시간 동기화 기록을 조회
- **후처리 지원**: EDF 파일과 웨어러블 데이터의 시간축 정렬을 위한 오프셋 계산

## 아키텍처

```
┌─────────────┐         WebSocket         ┌──────────────┐
│  PSG PC     │◄─────────────────────────►│              │
└─────────────┘                            │              │
                                           │  Time Sync   │
┌─────────────┐         WebSocket         │    Server    │
│ Galaxy Watch│◄─────────────────────────►│              │
└─────────────┘                            │              │
                                           └──────────────┘
                                                  │
                                                  ▼
                                            ┌──────────┐
                                            │ SQLite DB│
                                            └──────────┘
```

## 프로젝트 구조

```
time-sync-server/
├── cmd/
│   └── server/
│       └── main.go                 # 서버 엔트리포인트
├── internal/
│   ├── api/
│   │   ├── handler.go             # HTTP/WebSocket 핸들러
│   │   └── routes.go              # 라우팅 설정
│   ├── websocket/
│   │   ├── hub.go                 # WebSocket 연결 관리
│   │   └── client.go              # 클라이언트 처리
│   ├── algorithms/
│   │   ├── ntp_selector.go        # NTP 선택 알고리즘
│   │   └── ntp_selector_test.go   # 알고리즘 단위 테스트
│   ├── service/
│   │   ├── sync_service.go        # 비즈니스 로직
│   │   ├── auto_sync_monitor.go   # 자동 동기화 모니터
│   │   └── pairing_operator.go    # 페어링 자동 복구 모니터 
│   ├── repository/
│   │   └── sqlite.go              # DB 접근 레이어 (페어링 persistence 포함)
│   └── models/
│       ├── types.go               # 데이터 모델 (PersistentPairing 포함)
│       ├── measurement.go         # 측정값 처리
│       └── measurement_test.go    # 측정값 테스트
├── config/
│   └── config.go                  # 설정 관리
├── go.mod
├── go.sum
└── README.md
```

## 설치 및 실행

### 요구사항

- Go 1.25.0 이상

### 빌드

```bash
go build -o time-sync-server ./cmd/server
```

### 실행

```bash
# 기본 설정으로 실행 (포트 8080, DB: ./time-sync.db)
./time-sync-server

# 환경변수로 설정 변경
PORT=9000 DB_PATH=/path/to/database.db ./time-sync-server

# Auto-Sync 기본값 설정
AUTO_SYNC_INTERVAL_SEC=120 AUTO_SYNC_SAMPLE_COUNT=10 AUTO_SYNC_INTERVAL_MS=300 ./time-sync-server
```

### 개발 모드 실행

```bash
go run ./cmd/server/main.go
```

## API 사용법

### REST API

#### 1. 헬스 체크
```bash
GET /health
```

#### 2. 연결된 디바이스 조회
```bash
GET /api/devices
```

**응답 예시:**
```json
[
  {
    "deviceId": "psg-001",
    "deviceType": "PSG",
    "connectedAt": "2025-10-02T14:30:00Z"
  },
  {
    "deviceId": "watch-001",
    "deviceType": "WATCH",
    "connectedAt": "2025-10-02T14:31:00Z"
  }
]
```

#### 2-1. 디바이스 연결 건강도 조회

WebSocket 연결의 건강 상태를 모니터링할 수 있는 API입니다. PING/PONG 프로토콜을 통해 실시간으로 연결 상태를 추적합니다.

```bash
# 모든 디바이스의 건강도 조회
GET /api/devices/health

# 특정 디바이스의 건강도 조회
GET /api/devices/health?deviceId=psg-001
```

**응답 예시 (전체 조회):**
```json
[
  {
    "deviceId": "psg-001",
    "deviceType": "PSG",
    "connectedAt": "2025-10-18T14:30:00Z",
    "lastPingSent": "2025-10-18T14:35:20Z",
    "lastPongRecv": "2025-10-18T14:35:20Z",
    "lastRtt": 15,
    "isHealthy": true,
    "timeSinceLastPong": 5000
  },
  {
    "deviceId": "watch-001",
    "deviceType": "WATCH",
    "connectedAt": "2025-10-18T14:31:00Z",
    "lastPingSent": "2025-10-18T14:35:18Z",
    "lastPongRecv": "2025-10-18T14:35:18Z",
    "lastRtt": 25,
    "isHealthy": true,
    "timeSinceLastPong": 7000
  }
]
```

**응답 예시 (특정 디바이스 조회):**
```json
{
  "deviceId": "psg-001",
  "deviceType": "PSG",
  "connectedAt": "2025-10-18T14:30:00Z",
  "lastPingSent": "2025-10-18T14:35:20Z",
  "lastPongRecv": "2025-10-18T14:35:20Z",
  "lastRtt": 15,
  "isHealthy": true,
  "timeSinceLastPong": 5000
}
```

**응답 필드 설명:**

| 필드 | 타입 | 설명 |
|------|------|------|
| `deviceId` | string | 디바이스 ID |
| `deviceType` | string | 디바이스 타입 (PSG, WATCH, MOBILE) |
| `connectedAt` | timestamp | WebSocket 연결 시작 시간 (RFC3339) |
| `lastPingSent` | timestamp | 서버가 마지막으로 PING을 전송한 시간 |
| `lastPongRecv` | timestamp | 서버가 마지막으로 PONG을 수신한 시간 |
| `lastRtt` | int64 | 마지막 측정된 RTT (밀리초) |
| `isHealthy` | boolean | 연결 건강 상태 |
| `timeSinceLastPong` | int64 | 마지막 PONG 이후 경과 시간 (밀리초) |

**건강 상태 판정 기준:**
- `isHealthy: true` - 마지막 PONG 수신 후 **90초 이내**
- `isHealthy: false` - 마지막 PONG 수신 후 **90초 초과**
- **자동 연결 해제** - 마지막 PONG 수신 후 **120초 초과** (서버가 자동으로 연결 종료)

**사용 사례:**
```bash
# 주기적으로 연결 상태 확인 (모니터링 대시보드)
while true; do
  curl http://localhost:8080/api/devices/health | jq
  sleep 10
done

# 특정 디바이스가 건강한지 확인
curl "http://localhost:8080/api/devices/health?deviceId=psg-001" | jq '.isHealthy'
# 출력: true

# 비건강 디바이스 필터링 (jq 사용)
curl http://localhost:8080/api/devices/health | jq '.[] | select(.isHealthy == false)'
```

**에러 응답:**
```json
// 404 Not Found - 디바이스가 연결되지 않음
{
  "error": "device not connected: psg-001"
}
```

#### 3. 페어링 생성

페어링 생성 시 다음 작업이 자동으로 수행됩니다:
- **페어링 정보가 DB에 영구 저장** (디바이스 재연결 시 자동 복구 가능) 
- **Auto-Sync가 자동으로 시작** (백그라운드 실행)

Auto-Sync 설정은 선택적으로 지정 가능하며, 지정하지 않으면 서버의 기본값(환경변수)을 사용합니다.

```bash
POST /api/pairings
Content-Type: application/json

# 기본값으로 페어링 생성 (Auto-Sync 자동 시작)
{
  "device1Id": "psg-001",
  "device2Id": "watch-001"
}

# 커스텀 Auto-Sync 설정으로 페어링 생성
{
  "device1Id": "psg-002",
  "device2Id": "watch-002",
  "autoSyncIntervalSec": 120,
  "autoSyncSampleCount": 10,
  "autoSyncIntervalMs": 300
}
```

**요청 파라미터:**

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `device1Id` | string | ✅ | 첫 번째 디바이스 ID |
| `device2Id` | string | ✅ | 두 번째 디바이스 ID |
| `autoSyncIntervalSec` | int | ❌ | Auto-Sync 주기(초), 기본값: 환경변수 또는 600 |
| `autoSyncSampleCount` | int | ❌ | 샘플링 횟수, 기본값: 환경변수 또는 15 |
| `autoSyncIntervalMs` | int | ❌ | 샘플 간격(ms), 기본값: 환경변수 또는 200 |

**응답 예시:**
```json
{
  "pairingId": "550e8400-e29b-41d4-a716-446655440000"
}
```

**서버 로그:**
```
Auto-sync automatically started for pairing 550e8400-e29b-41d4-a716-446655440000 (interval: 120s, samples: 10, interval_ms: 300ms)
```

#### 4. 페어링 목록 조회

**DB에 저장된 모든 페어링**을 조회합니다 (in-memory가 아닌 영구 저장소 조회). 

```bash
GET /api/pairings
```

**특징:**
- 서버 재시작 후에도 페어링 목록 조회 가능
- 디바이스가 연결되지 않은 페어링도 포함

#### 5. 페어링 삭제

페어링 삭제 시 다음 작업이 자동으로 수행됩니다:
- **Auto-Sync 중지**
- **in-memory에서 삭제**
- **DB에서 영구 삭제**  (재연결 시 복구되지 않음)

```bash
DELETE /api/pairings/{pairingId}
```

**서버 로그:**
```
Auto-sync stopped for pairing 550e8400-e29b-41d4-a716-446655440000
Pairing deleted from DB
```

#### 6. 시간 동기화 실행 (단일 측정)
```bash
POST /api/sync/{pairingId}
```

**응답 예시:**
```json
{
  "success": true,
  "record": {
    "id": 1,
    "device1Id": "psg-001",
    "device1Type": "PSG",
    "device1Timestamp": 1727870400123,
    "device2Id": "watch-001",
    "device2Type": "WATCH",
    "device2Timestamp": 1727870400456,
    "serverRequestTime": 1727870400000,
    "serverResponseTime": 1727870401000,
    "device1Rtt": 5000,
    "device2Rtt": 8000,
    "timeDifference": -333,
    "status": "SUCCESS",
    "createdAt": 1727870401000
  }
}
```

**⚠️ 중요**: `timeDifference`는 **원본(raw) 오프셋**입니다 (네트워크 보정 없음). 단일 측정을 사용할 경우 다음과 같이 직접 보정해야 합니다:

```javascript
// 수동 네트워크 보정 (단일 측정용)
const rawOffset = record.timeDifference;        // -333ms
const delay1 = record.device1Rtt / 2 / 1000;    // 5000μs / 2 / 1000 = 2.5ms
const delay2 = record.device2Rtt / 2 / 1000;    // 8000μs / 2 / 1000 = 4ms
const adjustedOffset = rawOffset - (delay1 - delay2);
// -333 - (2.5 - 4) = -333 + 1.5 = -331.5ms
```

**권장**: 정확한 동기화를 위해서는 단일 측정 대신 **NTP 다중 샘플링**(아래)을 사용하세요.

#### 7. NTP 다중 샘플링 동기화 (권장)
```bash
POST /api/sync/multi
Content-Type: application/json

{
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "sample_count": 10,
  "interval_ms": 200,
  "timeout_sec": 5
}
```

**응답 예시:**
```json
{
  "success": true,
  "result": {
    "aggregation_id": "agg-uuid-xxx",
    "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
    "best_offset": -150,
    "median_offset": -150,
    "mean_offset": -151.2,
    "offset_std_dev": 3.5,
    "min_rtt": 5000,
    "max_rtt": 15000,
    "mean_rtt": 8500.0,
    "confidence": 0.94,
    "jitter": 2000.0,
    "total_samples": 10,
    "valid_samples": 8,
    "outlier_count": 2,
    "created_at": 1727870401000
  }
}
```

**파라미터 설명:**
- `sample_count`: 측정 횟수 (기본값: 8, 최대: 20)
- `interval_ms`: 측정 간격 밀리초 (기본값: 200ms)
- `timeout_sec`: 각 측정의 타임아웃 초 (기본값: 5초)

**응답 필드 설명:**
- `best_offset`: NTP 알고리즘으로 선택된 최적 시간 오프셋 (ms)
- `confidence`: 측정 신뢰도 점수 (0.0~1.0, 높을수록 신뢰도 높음)
- `jitter`: 네트워크 지연 변동성 (μs, 낮을수록 안정적)
- `offset_std_dev`: 오프셋 표준편차 (ms, 낮을수록 일관성 있음)

#### 8. 집계 결과 조회
```bash
# 전체 집계 결과 조회 (모든 페어링)
GET /api/sync/aggregated?limit=50&offset=0

# 특정 페어링의 NTP 결과 조회
GET /api/sync/aggregated?pairingId=550e8400-e29b-41d4-a716-446655440000&limit=50&offset=0

# 시간 범위로 집계 결과 조회 (RFC3339 형식)
GET /api/sync/aggregated?startTime=2025-10-01T00:00:00Z&endTime=2025-10-02T23:59:59Z&limit=50&offset=0

# 특정 집계 결과 상세 조회 (모든 개별 측정 포함)
GET /api/sync/aggregated/{aggregationId}
```

**쿼리 파라미터:**
- `pairingId` (선택): 특정 페어링으로 필터링
- `startTime`, `endTime` (선택): 시간 범위로 필터링 (RFC3339 형식)
- `limit` (선택): 조회할 결과 수 (기본값: 50, 최대: 1000)
- `offset` (선택): 페이지네이션 오프셋 (기본값: 0)

**응답 예시:**
```json
[
  {
    "aggregation_id": "agg-uuid-1",
    "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
    "best_offset": -150,
    "median_offset": -150,
    "mean_offset": -151.2,
    "confidence": 0.94,
    "created_at": 1727870401000
  },
  {
    "aggregation_id": "agg-uuid-2",
    "pairing_id": "550e8400-e29b-41d4-a716-446655440001",
    "best_offset": -182,
    "median_offset": -180,
    "mean_offset": -181.5,
    "confidence": 0.92,
    "created_at": 1727899201000
  }
]
```

#### 9. 동기화 이력 조회
```bash
# 전체 조회
GET /api/sync/records?limit=50&offset=0

# 특정 디바이스 조회
GET /api/sync/records?deviceId=psg-001&limit=50&offset=0

# 시간 범위로 조회
GET /api/sync/records?startTime=2025-10-01T00:00:00Z&endTime=2025-10-02T23:59:59Z&limit=50&offset=0

# 특정 record 상세 조회
GET /api/sync/records/{recordId}
```

**응답 예시 (상세 조회):**
```json
{
  "id": 123,
  "device1Id": "psg-001",
  "device1Type": "PSG",
  "device1Timestamp": 1727870400123,
  "device2Id": "watch-001",
  "device2Type": "WATCH",
  "device2Timestamp": 1727870400456,
  "serverRequestTime": 1727870400000,
  "serverResponseTime": 1727870401000,
  "device1Rtt": 5000,
  "device2Rtt": 8000,
  "timeDifference": -333,
  "status": "SUCCESS",
  "createdAt": 1727870401000
}
```

#### 10. Auto-Sync 관리

Auto-Sync는 페어링 생성 시 자동으로 시작되며, **시작 즉시 첫 동기화를 수행**한 후 설정된 주기마다 반복 실행됩니다. 수동으로 제어할 수도 있습니다.

##### 10-1. Auto-Sync 수동 시작

```bash
POST /api/auto-sync/start
Content-Type: application/json

{
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "interval_sec": 90,
  "sample_count": 12,
  "interval_ms": 250
}
```

**요청 파라미터:**

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `pairing_id` | string | ✅ | 페어링 ID |
| `interval_sec` | int | ❌ | 동기화 주기(초), 기본값: 600 |
| `sample_count` | int | ❌ | NTP 샘플 수, 기본값: 15 |
| `interval_ms` | int | ❌ | 샘플 간격(ms), 기본값: 200 |

**응답 예시:**
```json
{
  "message": "auto-sync started",
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**에러 응답:**
```json
// 이미 실행 중인 경우
{
  "error": "auto-sync already running for pairing: 550e8400-e29b-41d4-a716-446655440000"
}

// 페어링이 존재하지 않는 경우
{
  "error": "pairing not found: invalid-pairing-id"
}
```

##### 10-2. Auto-Sync 중지

```bash
POST /api/auto-sync/stop/{pairingId}
```

**예시:**
```bash
POST /api/auto-sync/stop/550e8400-e29b-41d4-a716-446655440000
```

**응답 예시:**
```json
{
  "message": "auto-sync stopped",
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**에러 응답:**
```json
{
  "error": "auto-sync not running for pairing: 550e8400-e29b-41d4-a716-446655440000"
}
```

##### 10-3. Auto-Sync 상태 조회

```bash
# 모든 Auto-Sync 작업 상태 조회
GET /api/auto-sync/status

# 특정 페어링의 Auto-Sync 상태 조회
GET /api/auto-sync/status?pairingId=550e8400-e29b-41d4-a716-446655440000
```

**응답 예시 (전체 조회):**
```json
{
  "jobs": [
    {
      "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "RUNNING",
      "config": {
        "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
        "interval_sec": 600,
        "sample_count": 15,
        "interval_ms": 200
      },
      "started_at": "2025-10-28T10:00:00Z",
      "last_sync_at": "2025-10-28T10:05:00Z",
      "last_sync_success": true,
      "last_error": "",
      "total_syncs": 5,
      "failed_syncs": 0
    },
    {
      "pairing_id": "another-pairing-id",
      "status": "RUNNING",
      "config": {
        "pairing_id": "another-pairing-id",
        "interval_sec": 120,
        "sample_count": 10,
        "interval_ms": 300
      },
      "started_at": "2025-10-28T10:02:00Z",
      "last_sync_at": "2025-10-28T10:04:00Z",
      "last_sync_success": false,
      "last_error": "timeout waiting for device response",
      "total_syncs": 2,
      "failed_syncs": 1
    }
  ]
}
```

**응답 예시 (특정 페어링 조회):**
```json
{
  "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "RUNNING",
  "config": {
    "pairing_id": "550e8400-e29b-41d4-a716-446655440000",
    "interval_sec": 600,
    "sample_count": 15,
    "interval_ms": 200
  },
  "started_at": "2025-10-28T10:00:00Z",
  "last_sync_at": "2025-10-28T10:05:00Z",
  "last_sync_success": true,
  "last_error": "",
  "total_syncs": 5,
  "failed_syncs": 0
}
```

**응답 필드 설명:**

| 필드 | 타입 | 설명 |
|------|------|------|
| `pairing_id` | string | 페어링 ID |
| `status` | string | 작업 상태 (RUNNING, STOPPED, FAILED) |
| `config` | object | Auto-Sync 설정 |
| `started_at` | timestamp | Auto-Sync 시작 시간 (RFC3339) |
| `last_sync_at` | timestamp | 마지막 동기화 시간 |
| `last_sync_success` | boolean | 마지막 동기화 성공 여부 |
| `last_error` | string | 마지막 에러 메시지 (있는 경우) |
| `total_syncs` | int | 총 동기화 시도 횟수 |
| `failed_syncs` | int | 실패한 동기화 횟수 |

**사용 사례:**

```bash
# 1. 실시간 모니터링 (watch 명령어)
watch -n 5 'curl -s http://localhost:8080/api/auto-sync/status | jq'

# 2. 특정 페어링 상태 확인
curl "http://localhost:8080/api/auto-sync/status?pairingId=pair-123" | jq

# 3. 실패한 작업 필터링 (jq 사용)
curl http://localhost:8080/api/auto-sync/status | jq '.jobs[] | select(.failed_syncs > 0)'

# 4. 성공률 계산
curl http://localhost:8080/api/auto-sync/status | jq '.jobs[] | {
  pairing_id,
  success_rate: ((.total_syncs - .failed_syncs) / .total_syncs * 100)
}'
```

### WebSocket 연결

#### 클라이언트 연결
```
ws://localhost:8080/ws?deviceType=PSG&deviceId=psg-001
ws://localhost:8080/ws?deviceType=WATCH&deviceId=watch-001
```

#### WebSocket 메시지 프로토콜

**서버 → 클라이언트: 연결 확인**
```json
{
  "type": "CONNECTED",
  "deviceId": "psg-001",
  "serverTime": 1727870400000
}
```

**서버 → 클라이언트: 시간 요청**
```json
{
  "type": "TIME_REQUEST",
  "requestId": "req-uuid-xxx",
  "pairingId": "pairing-uuid-xxx"
}
```

**클라이언트 → 서버: 시간 응답**
```json
{
  "type": "TIME_RESPONSE",
  "requestId": "req-uuid-xxx",
  "timestamp": 1727870400123
}
```

**서버 → 클라이언트: PING (연결 유지)**
```json
{
  "type": "PING",
  "timestamp": 1727870400000
}
```

**클라이언트 → 서버: PONG (연결 확인)**
```json
{
  "type": "PONG",
  "timestamp": 1727870400015
}
```

#### PING/PONG 연결 모니터링 프로토콜

서버는 **이중 PING 시스템**을 사용하여 WebSocket 연결 상태를 지속적으로 모니터링합니다.

##### 1️⃣ WebSocket 프로토콜 레벨 PING (네트워크 계층)

WebSocket 표준 프레임을 사용한 낮은 레벨의 연결 유지:

| 속성 | 값 |
|------|-----|
| **전송 주기** | 54초마다 |
| **프레임 타입** | WebSocket Ping Frame (opcode 0x9) |
| **처리 방식** | 브라우저/라이브러리가 자동으로 Pong 응답 |
| **타임아웃** | 60초 (Pong 미수신 시 연결 종료) |
| **목적** | 네트워크 계층 연결 유지, NAT/방화벽 세션 타임아웃 방지 |

**클라이언트 측 처리:**
- 대부분의 WebSocket 라이브러리에서 자동 처리
- 별도 구현 불필요 (브라우저/OS 레벨에서 자동 응답)

##### 2️⃣ 애플리케이션 레벨 PING/PONG (JSON 메시지)

애플리케이션 계층에서 명시적으로 연결 상태를 확인하고 RTT를 측정:

| 속성 | 값 |
|------|-----|
| **전송 주기** | 20초마다 |
| **메시지 형식** | JSON (`{"type": "PING", "timestamp": ...}`) |
| **처리 방식** | 클라이언트가 명시적으로 PONG 응답 필요 |
| **RTT 측정** | PING 전송 ~ PONG 수신 시간 차이 |
| **목적** | 연결 건강도 확인, RTT 측정, 애플리케이션 응답성 검증 |

**타임라인 예시:**
```
T=0s    : WebSocket 연결 수립
T=20s   : 서버 → PING (App-level)
T=20.015s: 클라이언트 → PONG (RTT: 15ms)
T=40s   : 서버 → PING
T=54s   : 서버 → Ping (Protocol-level)
T=54.002s: 클라이언트 → Pong (자동)
T=60s   : 서버 → PING (App-level)
...
```

##### 연결 상태 판정 기준

| 상태 | 조건 | 설명 |
|------|------|------|
| 🟢 **Healthy** | Last PONG < 90초 전 | 정상 연결, `isHealthy: true` |
| 🟡 **Unhealthy** | Last PONG > 90초 전 | 응답 지연, `isHealthy: false` |
| 🔴 **Dead** | Last PONG > 120초 전 | 자동 연결 해제 (30초마다 체크) |

**상태 전이:**
```
[연결 수립] → [Healthy]
              ↓ (90초 PONG 없음)
           [Unhealthy]
              ↓ (120초 PONG 없음)
           [Dead] → [연결 해제]
```

##### 클라이언트 구현 가이드

**필수 구현: PING에 대한 PONG 응답**

```javascript
const websocket = new WebSocket('ws://localhost:8080/ws?deviceId=psg-001&deviceType=PSG');

websocket.onmessage = (event) => {
  const message = JSON.parse(event.data);

  switch (message.type) {
    case 'PING':
      // ⚠️ 필수: PING 수신 시 즉시 PONG 응답
      websocket.send(JSON.stringify({
        type: 'PONG',
        timestamp: Date.now()
      }));
      console.log('Sent PONG response');
      break;

    case 'CONNECTED':
      console.log('Connected to server:', message);
      break;

    case 'TIME_REQUEST':
      // 시간 동기화 요청 처리
      handleTimeRequest(message);
      break;
  }
};

websocket.onerror = (error) => {
  console.error('WebSocket error:', error);
};

websocket.onclose = (event) => {
  console.log('WebSocket closed:', event.code, event.reason);
  // 120초 타임아웃으로 닫힌 경우: code 1006 (Abnormal Closure)
};
```

**권장 구현: 연결 건강도 모니터링**

```javascript
class HealthMonitor {
  constructor(websocket) {
    this.ws = websocket;
    this.lastPingReceived = Date.now();
    this.lastPongSent = Date.now();

    // 1분마다 건강도 체크
    setInterval(() => this.checkHealth(), 60000);
  }

  onPingReceived() {
    this.lastPingReceived = Date.now();

    // PONG 즉시 전송
    this.ws.send(JSON.stringify({
      type: 'PONG',
      timestamp: Date.now()
    }));
    this.lastPongSent = Date.now();
  }

  checkHealth() {
    const timeSinceLastPing = Date.now() - this.lastPingReceived;

    if (timeSinceLastPing > 90000) {
      console.warn('⚠️ Connection unhealthy: No PING for', timeSinceLastPing, 'ms');
      // 재연결 로직 실행 가능
    } else {
      console.log('✅ Connection healthy');
    }
  }

  async queryServerHealth() {
    // REST API로 서버 측 건강도 확인
    const response = await fetch('http://localhost:8080/api/devices/health?deviceId=psg-001');
    const health = await response.json();
    console.log('Server-side health:', health);
    return health;
  }
}

// 사용 예시
const monitor = new HealthMonitor(websocket);

websocket.onmessage = (event) => {
  const message = JSON.parse(event.data);

  if (message.type === 'PING') {
    monitor.onPingReceived();
  }
};
```

##### 연결 유지 모범 사례

**✅ DO (권장):**
- PING 수신 즉시 PONG 응답 (지연 최소화)
- 주기적으로 REST API로 연결 건강도 확인 (`/api/devices/health`)
- 120초 타임아웃 전에 재연결 로직 준비
- `onclose` 이벤트에서 자동 재연결 구현

**❌ DON'T (비권장):**
- PONG 응답 지연 (블로킹 작업 중 PING 무시)
- 프로토콜 레벨 PING만 의존 (애플리케이션 레벨 무시)
- 타임아웃 후 무한 재연결 시도 (백오프 전략 사용)

##### 디버깅 및 모니터링

**서버 로그 확인:**
```bash
# PING/PONG 관련 로그
tail -f server.log | grep -E "PING|PONG|Dead connection"

# 출력 예시:
# 2025/10/18 14:35:20 Received PONG from client psg-001, RTT: 15ms
# 2025/10/18 14:37:45 Dead connection detected: watch-001 (no PONG for 125s)
```

**REST API로 실시간 모니터링:**
```bash
# 실시간 건강도 모니터링 스크립트
watch -n 5 'curl -s http://localhost:8080/api/devices/health | jq'

# 또는 특정 디바이스만
watch -n 5 'curl -s "http://localhost:8080/api/devices/health?deviceId=psg-001" | jq'
```

**연결 해제 원인 분석:**
```javascript
websocket.onclose = (event) => {
  switch (event.code) {
    case 1000:
      console.log('Normal closure');
      break;
    case 1006:
      console.log('Abnormal closure - possibly 120s timeout');
      break;
    default:
      console.log('Connection closed:', event.code, event.reason);
  }
};
```

## NTP 다중 샘플링 알고리즘

### 개요

단일 측정은 네트워크 지터(jitter)와 일시적 지연에 취약합니다. NTP(Network Time Protocol) 표준에서 사용하는 다중 샘플링 기법을 적용하여 더 정확하고 신뢰할 수 있는 시간 오프셋을 계산합니다.

### NTPSelector 알고리즘 단계

NTP 표준 방식: **원본 수집 → 필터링 → 선택 → 보정**

```
10개 샘플 측정
    ↓
[Step 0] 원본 데이터 저장
    - Raw Offset = Device1Time - Device2Time (보정 없음)
    - RTT 정보 함께 저장
    ↓
[Step 1] RTT 필터링
    - 원본 오프셋과 RTT를 함께 분석
    - 총 RTT(Round-Trip Time) 기준 정렬
    - 상위 50% 선택 (낮은 지연 우선)
    - 원리: 낮은 RTT = 큐잉 지연 적음 = 더 정확
    ↓
[Step 2] 대칭성 필터링
    - |Device1RTT - Device2RTT| 작은 것 우선
    - 선택 점수 = TotalRTT + (RTTDifference × 2)
    - 원리: 대칭 경로 = 보정 신뢰도 높음
    ↓
[Step 3] 네트워크 지연 보정 적용
    - delay1 = Device1RTT / 2 (단방향 지연)
    - delay2 = Device2RTT / 2
    - adjustedOffset = rawOffset - (delay1 - delay2)
    - 원리: 네트워크 지연 차이 제거
    ↓
[Step 4] 이상값 제거
    - 보정된 오프셋의 평균 ± 2σ 벗어나면 제거
    - 최소 3개 샘플 유지
    ↓
[Step 5] 최종 계산
    - 중앙값(median) → best_offset
    - 평균, 표준편차, 신뢰도 계산
```

**중요**: 네트워크 보정은 필터링 **후**에 적용됩니다. 이렇게 하면 RTT 기반 필터링이 원본 데이터로 작동하여 더 정확한 샘플을 선택할 수 있습니다.

#### 네트워크 지연 보정 원리

```
예시: PSG와 Watch 사이의 시간 차이 측정

실제 상황:
PSG Time:   10:00:00.000
Watch Time: 10:00:00.150  (150ms 느림)

측정 과정:
1. Server → PSG 요청 전송 (5ms 소요)
2. PSG 응답: "10:00:00.000"
3. PSG → Server 응답 수신 (5ms 소요)
   → PSG RTT = 10ms

4. Server → Watch 요청 전송 (20ms 소요)
5. Watch 응답: "10:00:00.150"
6. Watch → Server 응답 수신 (20ms 소요)
   → Watch RTT = 40ms

원본 오프셋 계산:
rawOffset = PSG Time - Watch Time = 0 - 150 = -150ms

하지만 네트워크 지연 차이를 고려하면:
- PSG 단방향 지연: 10ms / 2 = 5ms
- Watch 단방향 지연: 40ms / 2 = 20ms
- 지연 차이: 5ms - 20ms = -15ms

보정된 오프셋:
adjustedOffset = -150 - (-15) = -150 + 15 = -135ms

→ PSG가 실제로 135ms 빠름 (네트워크 지연 효과 제거)
```

### 주요 메트릭 설명

#### 1. Jitter (네트워크 변동성)
```go
// RTT의 표준편차로 계산
jitter = sqrt(Σ(RTT_i - mean_RTT)² / N)
```

**의미**:
- **낮은 Jitter (< 1ms)**: 네트워크 안정적, 높은 신뢰도
- **높은 Jitter (> 10ms)**: 네트워크 불안정, 재측정 권장

**사용 목적**:
네트워크 품질을 정량화하여 시간 오프셋 계산의 신뢰성을 판단합니다. WiFi 환경에서는 jitter가 높을 수 있으며, 유선 연결에서는 낮습니다.

#### 2. Confidence Score (신뢰도 점수)
```
confidence = (샘플 개수 × 0.3) + (오프셋 일관성 × 0.4) + (네트워크 안정성 × 0.3)
```

**범위**: 0.0 ~ 1.0
- **0.9 이상**: 매우 신뢰할 수 있음
- **0.7 ~ 0.9**: 신뢰 가능
- **0.5 ~ 0.7**: 주의 필요
- **0.5 미만**: 재측정 권장

#### 3. Offset (시간 오프셋)
Device1 클럭이 Device2보다 얼마나 느린지/빠른지를 나타냅니다.
- **음수**: Device1이 Device2보다 느림 (예: -150ms = Device1이 150ms 뒤쳐짐)
- **양수**: Device1이 Device2보다 빠름

### 사용 시나리오: EDF 후처리

```bash
# EDF 측정 시작 직전
curl -X POST http://localhost:8080/api/sync/multi \
  -H "Content-Type: application/json" \
  -d '{
    "pairing_id": "psg-watch-pair",
    "sample_count": 10
  }'

# 응답: best_offset: -150ms, confidence: 0.94
# → PSG 클럭이 Watch보다 150ms 느림

# EDF 측정 종료 직후 (8시간 후)
curl -X POST http://localhost:8080/api/sync/multi \
  -H "Content-Type: application/json" \
  -d '{
    "pairing_id": "psg-watch-pair",
    "sample_count": 10
  }'

# 응답: best_offset: -182ms, confidence: 0.92
# → 8시간 동안 32ms 드리프트 발생
# → 추정 드리프트: -1.1 ppm (parts per million)
```

**후처리 적용**:
```python
# Python 예시: EDF와 Watch 데이터 시간축 정렬
edf_start_offset = -150  # ms
edf_end_offset = -182    # ms
duration = 8 * 3600 * 1000  # 8시간 (ms)

for edf_timestamp in edf_data:
    # 선형 보간으로 시간 보정
    elapsed = edf_timestamp - edf_start_time
    progress = elapsed / duration
    interpolated_offset = edf_start_offset + progress * (edf_end_offset - edf_start_offset)

    aligned_timestamp = edf_timestamp + interpolated_offset
    # Watch 데이터와 정렬된 타임스탬프 사용
```

### 알고리즘 검증

단위 테스트로 알고리즘 정확성을 검증합니다:
```bash
# NTP 알고리즘 테스트 실행
go test ./internal/algorithms/... -v

# 테스트 커버리지 확인
go test ./internal/algorithms/... -cover
# 결과: 89.9% coverage

# 주요 테스트 케이스:
# - RTT 필터링 (낮은 지연 우선)
# - 대칭성 필터링 (대칭 경로 우선)
# - 이상값 제거 (2σ 기준)
# - 네트워크 보정 적용 (원본 → 보정)
# - 신뢰도 계산 (샘플 품질 평가)
```

#### 네트워크 보정 테스트 예시

```go
// 동일한 원본 오프셋, 다른 RTT
Sample 1: Raw=-150ms, RTT1=5ms,  RTT2=6ms   → Adjusted=-149.5ms
Sample 2: Raw=-150ms, RTT1=20ms, RTT2=30ms  → Adjusted=-145ms

결과:
- Sample 1이 우선 선택됨 (낮은 RTT)
- 네트워크 보정이 올바르게 적용됨
- 최종 오프셋: -149.5ms (더 정확)
```

## 동작 흐름

1. **디바이스 연결**
   - PSG PC와 갤럭시 워치가 각각 WebSocket으로 서버에 연결
   - Query parameter로 deviceId와 deviceType 전달
   - **Pairing Operator가 디바이스 연결 감지** 

2. **페어링 생성 및 영구 저장** 
   - 관리자가 REST API로 두 디바이스를 페어링
   - **페어링 정보를 DB에 영구 저장** (디바이스 ID 조합, Auto-Sync 설정 포함)
   - 동시에 in-memory에도 등록 (빠른 접근)
   - **자동으로 Auto-Sync 시작** (백그라운드 goroutine에서 주기적 동기화 실행)

3. **디바이스 재연결 시 자동 복구** 
   - 디바이스가 재연결되면 **Pairing Operator가 자동으로 동작**
   - DB에서 해당 디바이스의 모든 페어링 조회
   - 상대 디바이스도 연결되어 있으면 **페어링 자동 복구**
   - 저장된 설정으로 **Auto-Sync 자동 재시작**
   - **Continuous 데이터 수집 가능** (네트워크 재연결에도 중단 없음)

4. **자동 시간 동기화 (Auto-Sync)**
   - 페어링별 독립적인 백그라운드 작업으로 실행
   - 시작 즉시 첫 동기화 수행, 이후 설정된 주기(기본 600초/10분)마다 반복 실행
   - NTP 다중 샘플링으로 정밀 측정 (기본 15회)
   - 동기화 결과는 자동으로 DB에 저장
   - 상태 API로 실시간 모니터링 가능

5. **수동 시간 동기화 (선택사항)**
   - **단일 측정**: REST API로 특정 페어링에 대해 1회 측정
   - **NTP 다중 샘플링**: 8-10회 측정 후 최적값 선택
   - 서버가 WebSocket을 통해 두 디바이스에게 시간 요청
   - 각 디바이스가 현재 시스템 시간을 응답
   - 서버가 결과를 DB에 저장

6. **이력 조회**
   - REST API로 저장된 동기화 기록 조회
   - NTP 집계 결과 및 개별 측정값 조회
   - Auto-Sync로 자동 수집된 데이터 포함

7. **페어링 삭제** (선택사항)
   - REST API로 페어링 완전 삭제
   - Auto-Sync 중지 → in-memory 삭제 → **DB에서도 삭제**
   - 재연결 시 복구되지 않음

## 환경 변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `PORT` | 서버 포트 | `8080` |
| `DB_PATH` | SQLite DB 파일 경로 | `./time-sync.db` |
| `AUTO_SYNC_INTERVAL_SEC` | Auto-Sync 기본 주기 (초) | `600` |
| `AUTO_SYNC_SAMPLE_COUNT` | Auto-Sync 기본 샘플 수 | `15` |
| `AUTO_SYNC_INTERVAL_MS` | Auto-Sync 샘플 간격 (ms) | `200` |

**사용 예시:**
```bash
# Auto-Sync 기본값을 커스터마이즈하여 서버 시작
AUTO_SYNC_INTERVAL_SEC=120 \
AUTO_SYNC_SAMPLE_COUNT=10 \
AUTO_SYNC_INTERVAL_MS=300 \
./time-sync-server

# 이후 페어링 생성 시 위 설정이 기본값으로 사용됨
```

## 의존성

- `github.com/gin-gonic/gin v1.10.0` - HTTP 웹 프레임워크
- `github.com/gorilla/websocket v1.5.3` - WebSocket 라이브러리
- `github.com/mattn/go-sqlite3 v1.14.22` - SQLite 드라이버
- `github.com/google/uuid v1.6.0` - UUID 생성

## 데이터베이스 스키마

### `time_sync_records` (개별 측정)
단일 시간 동기화 측정값을 저장합니다.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| id | INTEGER | Primary Key |
| device1_id | TEXT | Device 1 ID |
| device1_timestamp | INTEGER | Device 1 타임스탬프 (ms) |
| device1_rtt | INTEGER | Device 1 RTT (μs) |
| device2_id | TEXT | Device 2 ID |
| device2_timestamp | INTEGER | Device 2 타임스탬프 (ms) |
| device2_rtt | INTEGER | Device 2 RTT (μs) |
| time_difference | INTEGER | **원본** 시간 오프셋 (ms), 네트워크 보정 **없음** |
| status | TEXT | SUCCESS, PARTIAL, FAILED |
| created_at | INTEGER | 생성 시간 (ms) |

**중요**: `time_difference`는 원본(raw) 오프셋입니다. 네트워크 지연 보정은 NTPSelector가 다중 샘플링 시 적용합니다. 단일 측정 API를 사용할 경우 클라이언트가 RTT를 고려하여 직접 보정해야 합니다.

### `aggregated_sync_results` (NTP 집계 결과)
다중 샘플링 결과를 저장합니다. 모든 오프셋은 **네트워크 지연 보정이 적용된** 값입니다.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| aggregation_id | TEXT | Primary Key (UUID) |
| pairing_id | TEXT | 페어링 ID |
| best_offset | INTEGER | **최적 오프셋** (ms), 네트워크 보정 **적용됨** |
| median_offset | INTEGER | 중앙값 오프셋 (ms), 네트워크 보정 적용됨 |
| mean_offset | REAL | 평균 오프셋 (ms), 네트워크 보정 적용됨 |
| offset_std_dev | REAL | 오프셋 표준편차 (ms) |
| min_rtt | INTEGER | 최소 RTT (μs) |
| max_rtt | INTEGER | 최대 RTT (μs) |
| mean_rtt | REAL | 평균 RTT (μs) |
| confidence | REAL | 신뢰도 점수 (0.0~1.0) |
| jitter | REAL | 네트워크 변동성 (μs) |
| total_samples | INTEGER | 총 샘플 수 |
| valid_samples | INTEGER | 유효 샘플 수 |
| outlier_count | INTEGER | 제거된 이상값 개수 |
| created_at | INTEGER | 생성 시간 (ms) |

**권장**: EDF 후처리에는 `best_offset` 값을 사용하세요. 이 값은 NTP 알고리즘이 선택한 가장 신뢰할 수 있는 오프셋입니다.

### `pairings` (페어링 영구 저장) 
디바이스 페어링 정보와 Auto-Sync 설정을 영구 저장합니다.

| 컬럼 | 타입 | 설명 |
|------|------|------|
| pairing_id | TEXT | Primary Key (UUID) |
| device1_id | TEXT | Device 1 ID (NOT NULL) |
| device2_id | TEXT | Device 2 ID (NOT NULL) |
| created_at | INTEGER | 생성 시간 (ms) |
| auto_sync_interval_sec | INTEGER | Auto-Sync 주기 (초, NULL 가능) |
| auto_sync_sample_count | INTEGER | Auto-Sync 샘플 수 (NULL 가능) |
| auto_sync_interval_ms | INTEGER | Auto-Sync 샘플 간격 (ms, NULL 가능) |

**인덱스:**
- `idx_pairing_device1` - device1_id 인덱스
- `idx_pairing_device2` - device2_id 인덱스
- `idx_pairing_devices` - (device1_id, device2_id) UNIQUE 인덱스 (중복 방지)

**특징:**
- 페어링 정보가 **영구 저장**되어 서버 재시작 후에도 유지
- 디바이스 재연결 시 **자동 복구**에 사용됨
- Auto-Sync 설정도 함께 저장되어 복구 시 동일 설정으로 재시작

### `aggregation_measurements` (연결 테이블)
집계 결과와 개별 측정을 연결합니다.

## 사용 시나리오

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

# 2. 디바이스 연결 해제 (예: 네트워크 끊김)
# - in-memory 페어링 삭제
# - Auto-Sync 중단
# -  DB의 페어링은 그대로 유지

# 3. 디바이스 재연결 (WebSocket 재연결)
# 서버가 자동으로 감지하여:
# - DB에서 페어링 조회
# - 상대 디바이스 연결 확인
# - 페어링 자동 복구 (in-memory)
# - Auto-Sync 자동 재시작

# 서버 로그:
# "Client registered: watch-001 (WATCH)"
# "Found 1 pairing(s) for device watch-001, checking for restoration"
# "✓ Pairing restored: abc-123 (psg-001 <-> watch-001)"
# "✓ Auto-Sync automatically restarted for pairing abc-123 (interval: 600s, samples: 15)"

# 4. 상태 확인
curl http://localhost:8080/api/auto-sync/status?pairingId=abc-123 | jq

# 응답: Auto-Sync이 정상 실행 중임을 확인
```

**결과:**
- 네트워크 재연결 후 **수동 개입 없이** 자동으로 데이터 수집 재개
- Continuous 데이터 수집 보장
- 장시간 측정 시나리오에 최적화

### 시나리오 2: 서버 재시작 후 복구

서버를 재시작하더라도 페어링 정보가 DB에 저장되어 있어 복구 가능:

```bash
# 1. 서버 종료
# Ctrl+C 또는 kill 명령어

# 2. 서버 재시작
./bin/server

# 3. 디바이스들이 자동 재연결되면
# - Pairing Operator가 DB에서 페어링 조회
# - 자동으로 페어링 및 Auto-Sync 복구

# 4. 페어링 목록 확인
curl http://localhost:8080/api/pairings | jq

# 응답: DB에 저장된 모든 페어링 조회 가능
```

## 참고 문헌

- [RFC 5905: Network Time Protocol Version 4](https://datatracker.ietf.org/doc/html/rfc5905)
- NTP Clock Selection Algorithm
- IEEE 1588 Precision Time Protocol (PTP)

## 라이센스

MIT
