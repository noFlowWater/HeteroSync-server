# Research Proposal: Metamorphic Testing Framework for Oracle-less Physiological Signal Alignment Systems

## 1. Problem Definition
**HeteroSync** is a research framework designed to reconstruct a unified timeline for physiological signals (e.g., ECG, PPG) collected from independent, heterogeneous devices (e.g., PSG and smartwatches). The system relies on estimating clock offsets using Network Time Protocol (NTP) exchanges to align these disparate time series.

The core Software Engineering (SE) problem addressed in this proposal is the **"Test Oracle Problem"** regarding the verification of temporal alignment correctness. In a real-world deployment, the "ground truth" clock offset between two independent hardware clocks is physically unobservable. Consequently, when the system outputs an aligned dataset, there is no oracle to verify whether the estimated alignment is correct (e.g., validating if the offset is truly 123ms).

Furthermore, alignment faults in this domain do not result in immediate system crashes. Instead, they manifest as **"Silent Failures"**: the system runs successfully but produces output data where signals are subtly misaligned (e.g., by 500ms). These errors are difficult to detect using traditional unit testing methods that rely on fixed input-output assertions.

## 2. Motivation
Addressing the Oracle Problem in HeteroSync is critical for the following reasons:

1.  **Data Integrity for Downstream ML Models:** The aligned datasets are intended for delay-sensitive tasks such as Sleep Apnea detection or Heart Rate Variability (HRV) analysis. A "silent" alignment error can destroy the temporal correlation between channels (e.g., ECG and PPG), leading to **Garbage-In-Garbage-Output (GIGO)** in downstream Machine Learning models.
2.  **Inadequacy of Simulation-based Testing:** While simulation allows for testing with known ground truth, it cannot fully replicate the stochastic nature of hardware clock drifts and network jitters found in the wild. A testing strategy that verifies reliability in a production environment is essential.
3.  **Trustworthiness in Healthcare:** In medical software, "plausible but incorrect" data is more dangerous than a system crash. Establishing a rigorous testing framework to validate temporal consistency is a prerequisite for clinical adoption.

## 3. Background

### 3.1 Metamorphic Testing (MT)
Metamorphic Testing is a software testing technique effectively used to alleviate the "Oracle Problem." Instead of checking the correctness of an individual output against a ground truth (which is unavailable), MT verifies the relationships between multiple inputs and their corresponding outputs. These relationships, known as **Metamorphic Relations (MRs)**, define necessary properties of the target function. If an MR is violated, a fault definitely exists in the system.

### 3.2 Target System: HeteroSync Alignment Logic
The target system, HeteroSync, aligns physiological time series from heterogeneous devices (Type A: sparse timestamps, Type B: per-sample timestamps) using a Piecewise Linear Clock Model derived from NTP-like exchanges. The core logic involves detecting linear segments and estimating relative drift ($\gamma$) and offset ($\delta$) for each segment. Errors typically arise from incorrect segment boundary detection or noise-sensitive regression.

## 4. Proposed Approach
I propose a **Domain-Specific Metamorphic Testing Framework** for HeteroSync. This framework automatically generates test cases by transforming raw signal data and validating whether the alignment output satisfies predefined Metamorphic Relations (MRs).

### 4.1 Overview
The approach does not require a ground truth clock but relies on logical and physiological consistencies.

**Figure 1: Overview of the Proposed Testing Framework**
```text
[Source Inputs (Device A, B)] ----> [HeteroSync Alignment] ----> [Output O1]
        |                                                              ^
   (Transformation)                                                    | (Compare)
        v                                                              |
[Transformed Inputs (A', B')] --> [HeteroSync Alignment] ----> [Output O2]

          ===> CHECK: Does Relation(O1, O2) hold? (e.g., O2 == O1 + shift)
```
### 4.2 Defined Metamorphic Relations (MRs)
I define three categories of MRs to detect both logic errors and biological inconsistencies:

* **MR-1: Temporal Shift Consistency (Logic Verification)**
    * **Intuition:** If we artificially shift the input timestamps of Device B by a constant $\Delta t$, the calculated relative offset in the output should also shift by exactly $\Delta t$.
    * **Check:** $Alignment(T_A, T_B + \Delta t) \approx Alignment(T_A, T_B) + \Delta t$. A violation implies a bug in the regression or interpolation logic.

* **MR-2: Physio-Consistency (Domain Invariant Verification)**
    * **Intuition:** Biological signals must obey physiological laws. For instance, the electrical activation (ECG R-peak) must precede the mechanical pulse (PPG Systolic peak) by the Pulse Transit Time (PTT), typically 100–300ms.
    * **Check:** For the aligned dataset, the distribution of $(t_{PPG\_peak} - t_{ECG\_R\_peak})$ must fall within the biologically valid range $[100ms, 400ms]$. A negative value or excessive delay indicates a severe alignment failure (e.g., inverted signals).

* **MR-3: Clock Model Continuity (Model Stability)**
    * **Intuition:** System clocks may drift, but they rarely jump instantaneously unless an NTP step occurs. The estimated relative drift ($\gamma_k$) between adjacent segments should be continuous.
    * **Check:** $|\gamma_k - \gamma_{k+1}| < \epsilon$. Abrupt changes in drift estimation without a corresponding NTP event flag indicate a fault in the segment detection algorithm.

## 5. Expected Contribution
This work aims to provide the following contributions:

1.  **Reliable Validation without Ground Truth:** It presents a novel testing strategy for time-alignment systems where ground truth is fundamentally inaccessible, solving the Oracle Problem in the HeteroSync domain.
2.  **Preventing "Silent Failures" in Digital Health:** It contributes to **Data Quality Engineering** by filtering out physiologically inconsistent data before they reach downstream analysis.
3.  **Generalizable Framework:** The concept of "Physio-Consistency" (MR-2) can be generalized to other Cyber-Physical Systems (CPS) where multiple sensors observe correlated physical phenomena.

## 6. Evaluation Plan
To evaluate whether the proposed framework effectively addresses the problem, I will conduct a **Mutation Analysis** experiment.

* **Experimental Setup:**
    * **Dataset:** A mix of simulated data (with known ground truth for baseline validation) and real-world HeteroSync data.
    * **Mutant Generation (Fault Injection):** I will intentionally inject artificial faults into the HeteroSync alignment logic (e.g., inverting comparison operators, adding noise to NTP logs, forcing constant offsets) to mimic realistic bugs.
* **Metric:** The primary metric will be the **Mutation Score** (Fault Detection Rate):
    $$\text{Mutation Score} = \frac{\text{Number of Mutants Detected}}{\text{Total Number of Mutants Generated}}$$
* **Success Criteria:** A high mutation score will demonstrate that the proposed MRs are sensitive enough to catch subtle alignment bugs that traditional tests might miss.