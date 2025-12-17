# HeteroSync: Problem Definition

## 1. Research Objective

The goal of this research is to propose a **temporal alignment framework for physiological signals collected from independent heterogeneous devices**.

By estimating inter-device clock offsets and correcting temporal misalignment between channels, the system reconstructs multi-device signals **as if they were recorded on a single unified sensing platform**, ultimately producing an **Aligned Multimodal Physiological Dataset**.

### 1.1 Target Scenario

The concrete scenario assumes the following device pair:

| Device | Example | Timestamp Characteristics |
|--------|---------|---------------------------|
| **Device A** | PSG-connected computer | Only Start/End timestamps for channel data (relatively sparse) |
| **Device B** | Consumer wearable | Per-sample timestamps recorded by watch system clock |

### 1.2 Downstream Applications

The resulting Aligned Dataset can be directly used for:

- OSA event detection (AHI calculation)
- PPG → ECG reconstruction
- Sleep staging
- PPG morphological feature research
- HRV/BRV multi-device cross-validation

---

## 2. Input Specification

### 2.1 Input Data Types

Input data is categorized into two types based on timestamp information format:

#### Type A: Start-End Timestamp Only

- Only the **start time $t_{\text{start}}$** and **end time $t_{\text{end}}$** of channel data are recorded
- Each sample's time is estimated via interpolation under uniform sampling assumption:
  $$t_k = t_{\text{start}} + k \cdot \Delta t, \quad \Delta t = \frac{t_{\text{end}} - t_{\text{start}}}{N-1}$$
- **Example**: High-resolution signals from PSG-connected computer (EEG, EMG, ECG, etc.)

#### Type B: Per-Sample Timestamp

- Each data point has an **individual timestamp $t_k$** measured by the device system clock
- Time series format: $\{(t_k, x_k)\}_{k=1}^{N}$
- **Example**: PPG, accelerometer signals from consumer wearable devices

### 2.2 Timestamp Exchange Data

Additional data collected for alignment:

- **Periodic NTP-like timestamp exchange** records between server $S$ and each device
- Four timestamps per round-trip exchange:
  $$(S_1, C_i^2, C_i^3, S_4)$$
  - $S_1, S_4$: Send/receive times in server clock
  - $C_i^2, C_i^3$: Receive/send times in device $i$ clock

---

## 3. Output Specification

### 3.1 Aligned Multimodal Physiological Dataset

The final output is a dataset aligned under a **single Unified Timeline**:

| Component | Description |
|-----------|-------------|
| **PSG high-resolution signals** | EEG, EMG, ECG, SpO₂, etc. — realigned to unified timeline |
| **Wearable signals** | PPG, accelerometer, gyroscope, etc. — realigned to unified timeline |
| **Event labels** | PSG annotations (sleep stage, apnea events, etc.) — mapped to unified timeline |
| **Alignment quality metadata** | Per-segment estimated offset/drift, confidence metrics, parameters used |
| **Anomaly segment information** | NTP step/slew occurrence intervals, high-uncertainty alignment segments flagged |

### 3.2 Output Properties

- All signals exist on the **same time axis**
- Immediately usable for downstream analysis **as if measured from a single device**
- **Transparent metadata** on alignment quality enables reliability assessment

---

## 4. System Setting

### 4.1 Device Configuration

- Multiple heterogeneous devices $i \in \{1, \dots, N\}$ and a central server $S$
- All devices and server run OS-level **NTP clients**
- System clocks are **disciplined by NTP**, undergoing step/slew corrections

### 4.2 Clock Model

In an NTP-disciplined environment, each device's observable system clock $\tilde{C}_i(t)$:

- Is **not** a simple free-running linear clock
- Behaves as a **piecewise linear function** of true time $t$
- Exhibits discontinuities (step) or slope changes (slew) at NTP correction points

### 4.3 Timestamp Generation

Each device uses its local system clock $\tilde{C}_i(t)$ to timestamp physiological signals:

- **Type A device**: $\{t^i_{\text{start}}, t^i_{\text{end}}, \mathbf{x}^i\}$
- **Type B device**: $\{(t^i_k, x^i_k)\}_{k=1}^{N}$

---

## 5. Available Observations

### 5.1 NTP-like Timestamp Exchange Protocol

Server $S$ periodically (e.g., every 10 minutes) performs **application-layer NTP-like timestamp exchanges** with each device $i$.

Four timestamps recorded per round-trip exchange:

$$
(S_1, C_i^2, C_i^3, S_4)
$$

- $S_1$: Time server sent request (server clock)
- $C_i^2$: Time device received request (device clock)
- $C_i^3$: Time device sent response (device clock)
- $S_4$: Time server received response (server clock)

### 5.2 Derived Estimates

For each exchange sample $m$, NTP-style estimates:

$$
\theta_{i,m} = \frac{(C_i^2 - S_1) + (C_i^3 - S_4)}{2} \quad \text{(clock offset)}
$$

$$
d_{i,m} = (S_4 - S_1) - (C_i^3 - C_i^2) \quad \text{(round-trip delay)}
$$

### 5.3 Filtering and Aggregation

Within each time window $k$:

1. Apply RTT-based filtering
2. Apply path symmetry heuristics
3. Perform outlier rejection
4. Compute **robust representative offset** $b_{i,k}$
5. (Optional) Compute local drift estimate $a_{i,k}$

---

## 6. Problem Statement

### 6.1 Core Challenge

In an **NTP-disciplined environment** where all system clocks (devices and server) behave as piecewise linear functions,

the core problem is to reconstruct physiological signal time series recorded on different devices onto a **single Unified Timeline**.

### 6.2 Formal Problem Definition

**Given:**
- True reference time $t$
- Device $A$ observed timestamps: Type A or Type B format
- Device $B$ observed timestamps: Type A or Type B format
- NTP-disciplined, piecewise linear clocks: $\tilde{C}_A(t)$, $\tilde{C}_B(t)$

**Using:**
1. Periodic NTP-like timestamp exchanges between server and devices
2. Derived offset/delay estimate sequences

**Find:**

1. **Segment Detection**: Detect time intervals $k$ where the relative clock relationship is approximately linear (intervals between NTP step/slew events)

2. **Relative Clock Model Estimation**: For each segment $k$, estimate the relative clock model between devices $A$ and $B$:
   $$t_B \approx \gamma_k \cdot t_A + \delta_k$$
   - $\gamma_k$: relative drift (frequency ratio)
   - $\delta_k$: relative offset

3. **Time-Warping Function Construction**: Construct functions mapping each device's timeline to a common reference timeline:
   $$\Phi_{A \to \text{ref}}: t^A \mapsto t^{\text{ref}}$$
   $$\Phi_{B \to \text{ref}}: t^B \mapsto t^{\text{ref}}$$

### 6.3 Problem Summary

> Using repeated NTP-like timestamp exchanges with an NTP-disciplined central server, estimate a **piecewise linear relative clock model** $\{\gamma_k, \delta_k\}_k$ between heterogeneous devices,
>
> and leverage this model to realign multimodal physiological time series with **heterogeneous timestamp formats (Type A/B)** onto a **single Unified Timeline**,
>
> producing an **Aligned Multimodal Physiological Dataset** ready for downstream analysis.

---

## 7. Challenges & Considerations

### 7.1 Timestamp Type Heterogeneity

| Challenge | Description |
|-----------|-------------|
| Type A interpolation error | Sample times estimated from Start-End only cannot reflect actual sampling jitter |
| Type A/B alignment | Interaction between interpolation error and per-sample timestamp noise during alignment |
| Sampling rate mismatch | Resampling required when sampling rates differ across devices |

### 7.2 NTP-induced Discontinuities

| Challenge | Description |
|-----------|-------------|
| Step detection | Need to detect NTP step correction occurrence points |
| Slew rate change | Track slope changes during NTP slew mode |
| Segment boundary | Determine boundaries of piecewise linear segments |

### 7.3 Estimation Uncertainty

| Challenge | Description |
|-----------|-------------|
| Network asymmetry | Offset estimation error due to uplink/downlink delay asymmetry |
| Sparse observations | 10-minute interval exchanges may miss rapid changes |
| Clock quality variation | Differences in clock quality (stability, resolution) across devices |

---

## 8. Success Criteria

The final Aligned Multimodal Physiological Dataset must satisfy the following criteria:

1. **Temporal Coherence**: Physiologically related events (e.g., ECG R-peak and PPG systolic peak) are aligned within expected temporal relationships
2. **Alignment Quality Transparency**: Alignment confidence for each segment is provided as metadata
3. **Anomaly Flagging**: NTP correction intervals and high-uncertainty alignment segments are explicitly marked
4. **Downstream Usability**: Ready for ML pipeline ingestion without additional preprocessing