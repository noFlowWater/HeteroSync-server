# Time Sync Server

웨어러블 디바이스(갤럭시 워치)와 PSG 장비(Windows PC) 간의 시간 동기화를 위한 WebSocket 기반 서버입니다.

## 기능

- **WebSocket 연결 관리**: PSG PC와 갤럭시 워치가 WebSocket으로 서버에 연결
- **디바이스 페어링**: 연결된 디바이스들을 페어링하여 시간 동기화 준비
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
│   │   └── sync_service.go        # 비즈니스 로직
│   ├── repository/
│   │   └── sqlite.go              # DB 접근 레이어
│   └── models/
│       ├── types.go               # 데이터 모델
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

#### 3. 페어링 생성
```bash
POST /api/pairings
Content-Type: application/json

{
  "device1Id": "psg-001",
  "device2Id": "watch-001"
}
```

**응답 예시:**
```json
{
  "pairingId": "550e8400-e29b-41d4-a716-446655440000"
}
```

#### 4. 페어링 목록 조회
```bash
GET /api/pairings
```

#### 5. 페어링 삭제
```bash
DELETE /api/pairings/{pairingId}
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
# 특정 페어링의 NTP 결과 조회
GET /api/sync/aggregated?pairingId=550e8400-e29b-41d4-a716-446655440000&limit=50&offset=0

# 특정 집계 결과 상세 조회 (모든 개별 측정 포함)
GET /api/sync/aggregated/{aggregationId}
```

#### 9. 동기화 이력 조회
```bash
# 전체 조회
GET /api/sync/records?limit=50&offset=0

# 특정 디바이스 조회
GET /api/sync/records?deviceId=psg-001&limit=50&offset=0

# 시간 범위로 조회
GET /api/sync/records?startTime=2025-10-01T00:00:00Z&endTime=2025-10-02T23:59:59Z&limit=50&offset=0
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

2. **페어링 생성**
   - 관리자가 REST API로 두 디바이스를 페어링
   - 페어링 정보는 메모리에 저장 (휘발성)

3. **시간 동기화 실행**
   - **단일 측정**: REST API로 특정 페어링에 대해 1회 측정
   - **NTP 다중 샘플링** (권장): 8-10회 측정 후 최적값 선택
   - 서버가 WebSocket을 통해 두 디바이스에게 시간 요청
   - 각 디바이스가 현재 시스템 시간을 응답
   - 서버가 결과를 DB에 저장

4. **이력 조회**
   - REST API로 저장된 동기화 기록 조회
   - NTP 집계 결과 및 개별 측정값 조회

## 환경 변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `PORT` | 서버 포트 | `8080` |
| `DB_PATH` | SQLite DB 파일 경로 | `./time-sync.db` |

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

### `aggregation_measurements` (연결 테이블)
집계 결과와 개별 측정을 연결합니다.

## 참고 문헌

- [RFC 5905: Network Time Protocol Version 4](https://datatracker.ietf.org/doc/html/rfc5905)
- NTP Clock Selection Algorithm
- IEEE 1588 Precision Time Protocol (PTP)

## 라이센스

MIT
