# HeteroSync Server Architecture

## 프로젝트 구조

```
HeteroSync-server/
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
│   │   └── ntp_selector.go        # NTP 선택 알고리즘
│   ├── service/
│   │   ├── sync_service.go        # 비즈니스 로직
│   │   ├── auto_sync_monitor.go   # 자동 동기화 모니터
│   │   └── pairing_operator.go    # 페어링 자동 복구 모니터
│   ├── repository/
│   │   └── sqlite.go              # DB 접근 레이어
│   └── models/
│       └── types.go               # 데이터 모델
├── config/
│   └── config.go                  # 설정 관리
├── go.mod
├── go.sum
└── README.md
```

## 아키텍처 레이어

### 1. API Layer (`internal/api/`)
- HTTP 요청 처리 (Gin 프레임워크)
- WebSocket 연결 업그레이드
- 요청 검증 및 응답 포맷팅

### 2. Service Layer (`internal/service/`)
- **SyncService**: 시간 동기화 비즈니스 로직
  - 단일 측정 및 NTP 다중 샘플링
  - 결과 저장 및 조회
- **AutoSyncMonitor**: 자동 주기적 동기화
  - 백그라운드 작업 관리
  - 실시간 상태 추적
- **PairingOperator**: 페어링 자동 복구
  - 디바이스 재연결 감지
  - DB에서 페어링 복구

### 3. WebSocket Layer (`internal/websocket/`)
- **Hub**: 중앙 연결 관리자
  - 디바이스 등록/해제
  - 페어링 관리
  - 메시지 라우팅
  - 연결 상태 모니터링
- **Client**: 개별 WebSocket 연결
  - Read/Write 펌프
  - PING/PONG 처리
  - RTT 측정

### 4. Algorithm Layer (`internal/algorithms/`)
- **NTPSelector**: NTP 기반 오프셋 선택 알고리즘
  - RTT 필터링
  - 이상값 제거
  - 네트워크 지연 보정

### 5. Repository Layer (`internal/repository/`)
- SQLite 데이터베이스 접근
- CRUD 연산
- 트랜잭션 관리

---

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

### 네트워크 지연 보정 원리

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

### 알고리즘 검증

단위 테스트로 알고리즘 정확성을 검증합니다:
```bash
# NTP 알고리즘 테스트 실행
go test ./internal/algorithms/... -v

# 테스트 커버리지 확인
go test ./internal/algorithms/... -cover

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

---

## 동작 흐름

### 1. 디바이스 연결
- PSG와 Watch가 각각 WebSocket으로 서버에 연결
- Query parameter로 deviceId와 deviceType 전달
- **Pairing Operator가 디바이스 연결 감지**

### 2. 페어링 생성 및 영구 저장
- 관리자가 REST API로 두 디바이스를 페어링
- **페어링 정보를 DB에 영구 저장** (디바이스 ID 조합, Auto-Sync 설정 포함)
- 동시에 in-memory에도 등록 (빠른 접근)
- **자동으로 Auto-Sync 시작** (백그라운드 goroutine에서 주기적 동기화 실행)

### 3. 디바이스 재연결 시 자동 복구
- 디바이스가 재연결되면 **Pairing Operator가 자동으로 동작**
- DB에서 해당 디바이스의 모든 페어링 조회
- 상대 디바이스도 연결되어 있으면 **페어링 자동 복구**
- 저장된 설정으로 **Auto-Sync 자동 재시작**
- **Continuous 데이터 수집 가능** (네트워크 재연결에도 중단 없음)

### 4. 자동 시간 동기화 (Auto-Sync)
- 페어링별 독립적인 백그라운드 작업으로 실행
- 시작 즉시 첫 동기화 수행, 이후 설정된 주기(기본 600초/10분)마다 반복 실행
- NTP 다중 샘플링으로 정밀 측정 (기본 15회)
- 동기화 결과는 자동으로 DB에 저장
- 상태 API로 실시간 모니터링 가능

### 5. 수동 시간 동기화 (선택사항)
- **단일 측정**: REST API로 특정 페어링에 대해 1회 측정
- **NTP 다중 샘플링**: 8-10회 측정 후 최적값 선택
- 서버가 WebSocket을 통해 두 디바이스에게 시간 요청
- 각 디바이스가 현재 시스템 시간을 응답
- 서버가 결과를 DB에 저장

### 6. 이력 조회
- REST API로 저장된 동기화 기록 조회
- NTP 집계 결과 및 개별 측정값 조회
- Auto-Sync로 자동 수집된 데이터 포함

### 7. 페어링 삭제 (선택사항)
- REST API로 페어링 완전 삭제
- Auto-Sync 중지 → in-memory 삭제 → **DB에서도 삭제**
- 재연결 시 복구되지 않음

---

## 참고 문헌

- [RFC 5905: Network Time Protocol Version 4](https://datatracker.ietf.org/doc/html/rfc5905)
- NTP Clock Selection Algorithm
- IEEE 1588 Precision Time Protocol (PTP)
