# shellcheck shell=bash
# catalog.sh — 机型ID 命名规范表 (model-ID naming catalog)
#
# Sourced by catcher.bash. Defines, for each subsystem, a map from a
# *canonical key* (a normalized, hardware-independent description of the
# config) to the catalog ID, plus a reverse map ID -> human spec.
#
# Canonical-key conventions (must match what detect.sh produces):
#   CPU   "<Vendor> <SKU>|<sockets>"        e.g. "Intel 6530|2"
#   GPU   "<token>|<count>"                 token = family+variant+mem, e.g. "RTX5090|8"
#   MEM   "<sizeGB>|<dimm_count>"           e.g. "64|32"
#   NIC   "<spd>*<n>[+<spd>*<n>...]"        speeds in Gbps, segments sorted ASCENDING, merged per speed
#   DISK  "<label>*<n>[+...]"               labels normalized, segments sorted ASCENDING by capacity, merged per size
#
# Everything is data; matching logic lives in idgen.sh. Keep this file pure
# declarations so it is trivial to audit against the source spec.

# ---------------------------------------------------------------------------
# GPU   (token|count -> ID).  Token captures family + PCIe/SXM(nvlink) + mem,
# because e.g. A100-PCIe-40G, A100-PCIe-80G and A100-SXM-80G are different IDs.
# detect.sh maps the nvidia-smi product string to one of these tokens.
# ---------------------------------------------------------------------------
declare -gA GPU_ID=(
  ["RTX5090|8"]=G90        # 5090-PCIe-32GB *8
  ["B200|8"]=G92           # B200-nvlink181G *8
  ["B300|8"]=G94           # B300-nvlink288G *8
  ["L20|1"]=G80            # L20 *1
  ["H100-SXM-80G|8"]=G81   # H100-nvlink80G *8
  ["L40|8"]=G82            # L40 *8
  ["L40|4"]=G83            # L40 *4
  ["L40S|8"]=G84           # L40S *8
  ["L40S|4"]=G85           # L40S *4
  ["H20-96G|8"]=G86        # H20-nvlink96G *8
  ["H20-141G|8"]=G87       # H20-nvlink141G *8
  ["H200-141G|8"]=G88      # H200-nvlink141G *8
  ["A100-PCIE-40G|4"]=G70  # A100-PCIe40G *4
  ["A100-SXM-80G|8"]=G71   # A100-nvlink80G *8
  ["A100-SXM-40G|8"]=G72   # A100-nvlink40G *8
  ["A800-SXM-80G|8"]=G73   # A800-nvlink80G *8
  ["A100-PCIE-80G|8"]=G74  # A100-PCIe80G *8
  ["A100-PCIE-40G|8"]=G75  # A100-PCIe40G *8
  ["A100-PCIE-80G|4"]=G76  # A100-PCIe80G *4
  ["A10|1"]=G77            # A10 *1
  ["A10|8"]=G78            # A10 *8
  ["A10|4"]=G79  	   # A10 *4
  ["A100-PCIE-40G|1"]=G7A
  ["RTX3090|4"]=G7B
  ["V100-PCIE-16G|1"]=G50  # V100-PCIe-16G *1
  ["V100-SXM2|8"]=G51      # V100-SXM2 *8
  ["V100S-PCIE-32G|6"]=G53 # V100S-PCIe 32G *6
  ["V100S-PCIE-32G|1"]=G54 # V100S-PCIe 32G *1
  ["2080Ti|8"]=G61         # 2080Ti *8
)
declare -gA GPU_DESC=(
  [G90]="5090 *8"      [G92]="B200 *8"     [G94]="B300 *8"
  [G80]="L20 *1"       [G81]="H100 *8"     [G82]="L40 *8"
  [G83]="L40 *4"       [G84]="L40S *8"     [G85]="L40S *4"
  [G86]="H20 96G *8"   [G87]="H20 141G *8" [G88]="H200 *8"
  [G70]="A100-PCIe 40G *4" [G71]="A100 80G *8" [G72]="A100 40G *8"
  [G73]="A800 80G *8"  [G74]="A100-PCIe 80G *8" [G75]="A100-PCIe 40G *8"
  [G76]="A100-PCIe 80G *4" [G77]="A10 *1"  [G78]="A10 *8" [G79]="A10 *4"
  [G7A]="A100-PCIe 40G *1" [G7B]="3090 *4"
  [G50]="V100-PCIe 16G *1" [G51]="V100-SXM2 *8" [G53]="V100S-PCIe 32G *6" [G54]="V100S-PCIe 32G *1"
  [G61]="2080Ti *8"
)

# ---------------------------------------------------------------------------
# CPU   ("<Vendor> <SKU>|<sockets>" -> ID)
# ---------------------------------------------------------------------------
declare -gA CPU_ID=(
  ["Intel 6745P|2"]=I61 ["Intel 6767P|2"]=I62 ["Intel 6530P|2"]=I63
  ["Intel 6960P|2"]=I64 ["Intel 6982P|1"]=I65 ["Intel 6982P|2"]=I66
  ["Intel 6776P|2"]=I67 ["Intel 6768P|2"]=I68
  ["Intel 8558P|2"]=I51 ["Intel 5520+|2"]=I52 ["Intel 4516Y+|2"]=I53
  ["Intel 6548Y+|2"]=I54 ["Intel 8562Y+|2"]=I55 ["Intel 8558|2"]=I56
  ["Intel 6530|2"]=I57 ["Intel 8575C|2"]=I58
  ["Intel 6430|2"]=I41 ["Intel 8468|2"]=I42 ["Intel 8468V|2"]=I43
  ["Intel 6444Y|2"]=I44 ["Intel 5418Y|2"]=I45 ["Intel 6418H|2"]=I46
  ["Intel 6448H|2"]=I47 ["Intel 8480+|2"]=I48 ["Intel 8457C|2"]=I49 ["Intel 6459C|2"]=I4A
  ["Intel 8358|2"]=I31 ["Intel 6330|2"]=I32 ["Intel 8358P|2"]=I33
  ["Intel 6348|2"]=I34 ["Intel 8374C|2"]=I35 ["Intel 6326|2"]=I36
  ["Intel 6354|2"]=I37 ["Intel 5318Y|2"]=I38 ["Intel 6346|2"]=I39
  ["Intel 6342|2"]=I3A ["Intel 4314|2"]=I30 ["Intel 6130|4"]=I11
  ["Intel 6230|2"]=I21 ["Intel 4210|2"]=I22 ["Intel 6226R|2"]=I23
  ["Intel 4214|2"]=I24 ["Intel 6258R|2"]=I25 ["Intel 6254|4"]=I26 ["Intel 6248R|2"]=I27
  ["AMD 9575F|2"]=A51 ["AMD 9375F|2"]=A52 ["AMD 9755|2"]=A53
  ["AMD 9335|1"]=A54
  ["AMD 9354|2"]=A41 ["AMD 9334|2"]=A42 ["AMD 9274F|2"]=A43 ["AMD 9374F|2"]=A44
  ["AMD 9454P|1"]=A45
  ["AMD 7713|2"]=A31 ["AMD 74F3|2"]=A32 ["AMD 7443|2"]=A33 ["AMD 7773X|2"]=A34 ["AMD 75F3|2"]=A35
  ["AMD 7352|2"]=A21 ["AMD 7402|2"]=A22 ["AMD 7742|2"]=A23 ["AMD 7702|2"]=A24
  ["AMD 7H12|2"]=A25 ["AMD 7452|2"]=A26
)
declare -gA CPU_DESC=(
  [I61]="Intel 6745P *2" [I62]="Intel 6767P *2" [I63]="Intel 6530P *2"
  [I64]="Intel 6960P *2" [I65]="Intel 6982P *1" [I66]="Intel 6982P *2"
  [I67]="Intel 6776P *2" [I68]="Intel 6768P *2"
  [I51]="Intel 8558P *2" [I52]="Intel 5520+ *2" [I53]="Intel 4516Y+ *2"
  [I54]="Intel 6548Y+ *2" [I55]="Intel 8562Y+ *2" [I56]="Intel 8558 *2"
  [I57]="Intel 6530 *2" [I58]="Intel 8575C *2"
  [I41]="Intel 6430 *2" [I42]="Intel 8468 *2" [I43]="Intel 8468V *2"
  [I44]="Intel 6444Y *2" [I45]="Intel 5418Y *2" [I46]="Intel 6418H *2"
  [I47]="Intel 6448H *2" [I48]="Intel 8480+ *2" [I49]="Intel 8457C *2" [I4A]="Intel 6459C *2"
  [I31]="Intel 8358 *2" [I32]="Intel 6330 *2" [I33]="Intel 8358P *2"
  [I34]="Intel 6348 *2" [I35]="Intel 8374C *2" [I36]="Intel 6326 *2"
  [I37]="Intel 6354 *2" [I38]="Intel 5318Y *2" [I39]="Intel 6346 *2"
  [I3A]="Intel 6342 *2" [I30]="Intel 4314 *2" [I11]="Intel 6130 *4"
  [I21]="Intel 6230 *2" [I22]="Intel 4210 *2" [I23]="Intel 6226R *2"
  [I24]="Intel 4214 *2" [I25]="Intel 6258R *2" [I26]="Intel 6254 *4" [I27]="Intel 6248R *2"
  [A51]="AMD 9575F *2" [A52]="AMD 9375F *2" [A53]="AMD 9755 *2"
  [A54]="AMD 9335 *1"
  [A41]="AMD 9354 *2" [A42]="AMD 9334 *2" [A43]="AMD 9274F *2" [A44]="AMD 9374F *2"
  [A45]="AMD 9454P *1"
  [A31]="AMD 7713 *2" [A32]="AMD 74F3 *2" [A33]="AMD 7443 *2" [A34]="AMD 7773X *2" [A35]="AMD 75F3 *2"
  [A21]="AMD 7352 *2" [A22]="AMD 7402 *2" [A23]="AMD 7742 *2" [A24]="AMD 7702 *2"
  [A25]="AMD 7H12 *2" [A26]="AMD 7452 *2"
)

# CPU SKU -> memory generation (DDR4/DDR5). Used so describe-from-ID can print
# "DDR4 64G*32" even though the M-code itself doesn't encode the DDR gen.
# Rule of thumb: Cascade Lake (62xx/52xx/42xx) & Ice Lake (I3x) & AMD Rome/Milan
# (7xxx) are DDR4; Sapphire/Emerald/Granite Rapids (I4x/I5x/I6x) & AMD Genoa/
# Turin (9xxx) are DDR5. detect mode overrides this with the real dmidecode type.
declare -gA CPU_MEMGEN=(
  [I61]=DDR5 [I62]=DDR5 [I63]=DDR5 [I64]=DDR5 [I65]=DDR5 [I66]=DDR5 [I67]=DDR5 [I68]=DDR5
  [I51]=DDR5 [I52]=DDR5 [I53]=DDR5 [I54]=DDR5 [I55]=DDR5 [I56]=DDR5 [I57]=DDR5 [I58]=DDR5
  [I41]=DDR5 [I42]=DDR5 [I43]=DDR5 [I44]=DDR5 [I45]=DDR5 [I46]=DDR5 [I47]=DDR5 [I48]=DDR5 [I49]=DDR5 [I4A]=DDR5
  [I30]=DDR4 [I31]=DDR4 [I32]=DDR4 [I33]=DDR4 [I34]=DDR4 [I35]=DDR4 [I36]=DDR4 [I37]=DDR4 [I38]=DDR4 [I39]=DDR4 [I3A]=DDR4
  [I21]=DDR4 [I22]=DDR4 [I23]=DDR4 [I24]=DDR4 [I25]=DDR4 [I26]=DDR4 [I27]=DDR4 [I11]=DDR3
  [A51]=DDR5 [A52]=DDR5 [A53]=DDR5 [A54]=DDR5 [A41]=DDR5 [A42]=DDR5 [A43]=DDR5 [A44]=DDR5 [A45]=DDR5
  [A31]=DDR4 [A32]=DDR4 [A33]=DDR4 [A34]=DDR4 [A35]=DDR4 [A21]=DDR4 [A22]=DDR4 [A23]=DDR4 [A24]=DDR4 [A25]=DDR4 [A26]=DDR4
)

# ---------------------------------------------------------------------------
# Memory   ("<sizeGB>|<count>" -> ID)
# NOTE: the source spec lists M60 and M61 with the same config (64G*8). Per the
# catalog owner, M61 is the canonical code and M60 has been dropped entirely.
# ---------------------------------------------------------------------------
declare -gA MEM_ID=(
  ["8|3"]=M31  ["8|4"]=M32
  ["16|10"]=M41 ["16|16"]=M42
  ["32|8"]=M51 ["32|16"]=M52 ["32|24"]=M53 ["32|32"]=M54 ["32|12"]=M55
  ["64|8"]=M61 ["64|12"]=M62 ["64|16"]=M63 ["64|24"]=M64 ["64|32"]=M65 ["64|28"]=M66 ["64|48"]=M67
  ["96|32"]=M75 ["96|24"]=M76
)
declare -gA MEM_DESC=(
  [M31]="8G *3"  [M32]="8G *4"
  [M41]="16G *10" [M42]="16G *16"
  [M51]="32G *8" [M52]="32G *16" [M53]="32G *24" [M54]="32G *32" [M55]="32G *12"
  [M61]="64G *8" [M62]="64G *12" [M63]="64G *16" [M64]="64G *24" [M65]="64G *32" [M66]="64G *28" [M67]="64G *48"
  [M75]="96G *32" [M76]="96G *24"
)

# ---------------------------------------------------------------------------
# Network   (canonical speed-group -> ID).  Speeds in Gbps, ascending, merged.
# New entries map source rows written as port-groups ("25G *4") to the
# canonical "<spd>Gx<ports>*<cards>" form: 25G*4 -> 25Gx2*2.
# ---------------------------------------------------------------------------
declare -gA NIC_ID=(
  ["10Gx2*1"]=N11
  ["25Gx2*1"]=N12
  ["100Gx2*1"]=N13
  ["200Gx2*1"]=N14
  ["400Gx2*1"]=N15
  ["400Gx4*1"]=N16
  ["25Gx2*1+200Gx1*1"]=N20
  ["10Gx2*1+100Gx2*1"]=N21
  ["10Gx2*1+200Gx2*1"]=N22
  ["25Gx2*1+200Gx2*1"]=N23
  ["25Gx2*1+400Gx2*1"]=N24
  ["25Gx2*1+400Gx4*1"]=N25
  ["25Gx2*1+100Gx2*1"]=N26
  ["25Gx2*1+400Gx8*1"]=N27
  ["25Gx2*1+400Gx3*1"]=N28
  ["25Gx2*1+200Gx4*1"]=N29
  ["25Gx2*1+100Gx8*1"]=N2A
  ["100Gx2*1+200Gx2*1"]=N2B
  ["100Gx2*1+400Gx4*1"]=N2C
  ["400Gx2*1+800Gx1*1"]=N2D
  ["10Gx2*1+200Gx4*1"]=N2E
  ["100Gx3*1+200Gx4*1"]=N2F
  ["25Gx2*1+200Gx2*1+400Gx8*1"]=N31
  ["100Gx6*1+200Gx18*1"]=N32             # source: 100Gbps*6 +200Gbps *18
  ["100Gx2*1+100Gx4*1+200Gx2*1+200Gx4*8"]=N33
  ["25Gx2*1+200Gx1*1+400Gx8*1"]=N34
  ["25Gx2*1+100Gx2*1+200Gx2*1"]=N36
  ["25Gx2*1+100Gx2*2+200Gx2*1"]=N37
  ["25Gx2*1+100Gx2*4+200Gx2*1"]=N38
  ["25Gx4*1+200Gx4*1"]=N40
  ["1Gx2*1+10Gx2*1+10Gx2*1+10Gx2*1"]=N41
  ["1Gx2*1+10Gx2*1+25Gx4*1"]=N42
  ["25Gx2*1+10Gx2*1"]=N43
)
declare -gA NIC_DESC=(
  [N11]="10Gbps *2"
  [N12]="25Gbps *2"
  [N13]="100Gbps *2"
  [N14]="200Gbps *2"
  [N15]="400Gbps *2"
  [N16]="400Gbps *4"
  [N20]="25Gbps *2 + 200Gbps *1"
  [N21]="10Gbps *2 + 100Gbps *2"
  [N22]="10Gbps *2 + 200Gbps * 2"
  [N23]="25Gbps *2 + 200Gbps * 2"
  [N24]="25Gbps *2 + 400Gbps * 2"
  [N25]="25Gbps *2 + 400Gbps * 4"
  [N26]="25Gbps *2 + 100Gbps *2"
  [N27]="25Gbps *2 + 400Gbps *8"
  [N28]="25Gbps *2 + 400Gbps *3"
  [N29]="25Gbps *2 + 200Gbps *4"
  [N2A]="25Gbps *2 + 100Gbps *8"
  [N2B]="100Gbps*2 + 200Gbps * 2"
  [N2C]="100Gbps*2 + 400Gbps * 4"
  [N2D]="400Gbps*2 + 800Gbps * 1"
  [N2E]="10Gbps*2 + 200Gbps * 4"
  [N2F]="100Gbps*3 + 200Gbps * 4"
  [N31]="25Gbps *2 + 200G*2 + 400Gbps *8"
  [N32]="100Gbps *6 + 200Gbps *18"
  [N33]="100Gx2*1+100Gx4*1+200Gx2*1+200Gx4*8"
  [N34]="25Gbps *2 + 200G*1 + 400Gbps * 8"
  [N36]="25Gbps x2*1 + 100G x2*1 + 200Gbps x2*1"
  [N37]="25Gbps x2*1 + 100G x2*2 + 200Gbps x2*1"
  [N38]="25Gbps x2*1 + 100G x2*4 + 200Gbps x2*1"
  [N40]="25G *4 + 200G *4"
  [N41]="1Gbps*2+10Gbps*2+10Gbps*2+10Gbps*2"
  [N42]="1Gbps*2+10Gbps*2+25Gbps*4"
  [N43]="25Gbps*2 +10Gbps *2"
)
# Per-ID InfiniBand generation override for the high-speed ports, where the
# source examples pin it (200G can be HDR *or* NDR200). Key "<Nid>:<spd>".
# Anything not listed falls back to NIC_IBGEN_DEFAULT by speed in idgen.sh.
declare -gA NIC_IBGEN=(
  ["N20:200"]=NDR   # example: I31G82N20... -> "NDR 200G单口网卡*1"
  ["N23:200"]=HDR   # example: I31G82N23... -> "HDR 200G单口网卡*2"
)

# ---------------------------------------------------------------------------
# Disk   (canonical tier-group -> ID).  Sizes normalized, segments ascending by
# capacity and merged per size. Media (SSD/HDD) is descriptive only (kept in
# DISK_DESC); matching is by size+count because that is all most IDs encode.
# ---------------------------------------------------------------------------
declare -gA DISK_ID=(
  ["480G*1"]=S01
  ["480G*2"]=S02
  ["960G*2"]=S03
  ["1.2T*2"]=S04
  ["800G*2"]=S05
  ["1.6T*1"]=S06               # source: 1.6TB *1
  ["1.7T*1"]=S06               # alias: same capacity class as 1.6T (bucket 1700)
  ["3.84T*8"]=S07              # source: 3.84TB *8
  ["3.84T*10"]=S08             # source: 3.84TB *10
  ["1.92T*2"]=S09
  ["2.4T*1"]=S0A
  ["3.84T*1"]=S0B
  ["960G*2+3.84T*2"]=S10
  ["480G*2+1.92T*3"]=S11
  ["960G*2+1.92T*2"]=S12
  ["960G*2+3.84T*4"]=S13
  ["960G*2+3.84T*3"]=S14
  ["960G*2+3.84T*6"]=S15
  ["960G*2+10T*2"]=S16
  ["960G*2+3.84T*10"]=S17
  ["1.92T*2+3.84T*8"]=S18
  ["960G*2+1.92T*6"]=S19
  ["2.4T*8+3.84T*3"]=S1A
  ["960G*2+12T*12"]=S1B
  ["960G*2+7.68T*16"]=S1C
  ["960G*2+7.68T*10"]=S1D
  ["960G*2+3.84T*8"]=S1F
  ["960G*2+7.68T*4"]=S20
  ["960G*2+7.68T*4+20T*36"]=S21
  ["960G*2+2.4T*8+3.84T*6"]=S22           # NOTE: source lists S23 twice; see S23b
  ["960G*2+7.68T*8+20T*60"]=S23           # second S23 variant -> same ID
  ["800G*2+2.4T*6+3.84T*4"]=S24
  ["1.2T*1+3.84T*1"]=S25
  ["1.2T*3+3.84T*3+7.68T*3"]=S26
  ["1.2T*3+1.92T*4"]=S27                  # source: 1.2TB*3 + 1.92TB*2 + 1.92TB*2 ; merged 1.92T*4
  ["480G*2+2.4T*8+3.84T*4"]=S28
  ["2.4T*8+3.84T*8"]=S29
  ["800G*2+2.4T*4+3.84T*2"]=S2A
  ["960G*2+7.68T*4+20T*18"]=S2B
  ["480G*1+3.84T*2"]=S30
  ["480G*1+3.84T*4"]=S31
  ["1.2T*1+2.4T*1"]=S33
  ["960G*1+2.4T*1"]=S34
  ["480G*1+2.4T*1"]=S35
  ["960G*1+12T*1"]=S36
  ["600G*3+2.4T*16"]=S37
  ["960G*1+3.84T*1"]=S38
  ["480G*1+3.84T*3"]=S39
  ["960G*1+1.92T*1"]=S3A
  ["800G*1+3.84T*1+7.68T*1"]=S3B
  ["800G*1+2.4T*1"]=S3C
  ["480G*1+3.84T*2+4T*1"]=S3D
  ["960G*4+7.68T*2"]=S3E
  ["960G*1+3.84T*1+7.68T*1"]=S3F
  ["960G*1+1.92T*1+3.84T*2"]=S40
  ["960G*1+3.84T*1+12T*1"]=S41
  ["960G*1+1.92T*1+3.84T*1"]=S42
  ["960G*1+7.68T*1"]=S43
  ["2.4T*1+12T*1"]=S44
  ["480G*2+7.68T*16"]=S45
  ["1.92T*3+3.84T*3+7.68T*3"]=S46
  ["480G*2+7.68T*2+18T*36"]=S47
  ["960G*2+3.84T*1"]=S48
)
declare -gA DISK_DESC=(
  [S01]="480G *1" [S02]="480G *2" [S03]="960G *2" [S04]="1.2T *2" [S05]="800G *2"
  [S06]="1.6TB *1" [S07]="3.84TB *8" [S08]="3.84TB *10" [S09]="1.92TB *2" [S0A]="2.4T *1" [S0B]="3.84T *1"
  [S10]="960G *2 + 3.84T *2" [S11]="480G *2 + 1.92T *3" [S12]="960G *2 + 1.92T *2"
  [S13]="960G *2 + 3.84T *4" [S14]="960G *2 + 3.84T *3" [S15]="960G *2 + 3.84T *6"
  [S16]="960G *2 + 10T *2" [S17]="960G *2 + 3.84T *10" [S18]="1.92T *2 + 3.84T *8"
  [S19]="960G *2 + 1.92T *6" [S1A]="3.84T *3 + 2.4T *8" [S1B]="960G *2 + 12T *12"
  [S1C]="960G *2 + 7.68T *16" [S1D]="960G *2 + 7.68T *10" [S1F]="960G *2 + 3.84T *8"
  [S20]="960G *2 + 7.68T *4"
  [S21]="960G *2 + 7.68T *4 + 20T *36" [S22]="960G *2 + 3.84T *6 + 2.4T *8"
  [S23]="960G *2 + 7.68T *8 + 20T *60" [S24]="800G *2 + 2.4T *6 + 3.84T *4"
  [S2B]="960G *2 + 7.68T *4 + 20T *18"
  [S25]="1.2TB *1 + 3.84TB *1" [S26]="1.2TB *3 + 3.84TB *3 + 7.68TB *3"
  [S27]="1.2TB *3 + 1.92TB *2 + 1.92TB *2"
  [S28]="480GB *2 + SSD 3.84T *4 + HDD 2.4T *8"
  [S29]="HDD 2.4T *8 + SSD 3.84T *8" [S2A]="800G *2 + SSD 3.84T *2 + HDD 2.4T *4"
  [S30]="480G *1 + 3.84T *2" [S31]="480G *1 + 3.84T *4"
  [S33]="1.2T *1 + 2.4T *1"
  [S34]="960G *1 + 2.4T *1" [S35]="480G *1 + 2.4T *1"
  [S36]="960G *1 + 12T *1" [S37]="600G *3 + 2.4T *16"
  [S38]="960G *1 + 3.84T *1" [S39]="480G *1 + 3.84T *3" [S3A]="960G *1 + 1.92T *1"
  [S3B]="800G *1 + 3.84T *1 + 7.68T *1" [S3C]="800G *1 + 2.4T *1"
  [S3D]="480G *1 + 3.84T *2 + 4T *1" [S3E]="960G *4 + 7.68T *2"
  [S3F]="960G *1 + 3.84T *1 + 7.68T *1" [S40]="960G *1 + 1.92T *1 + 3.84T *2"
  [S41]="960G *1 + 3.84T *1 + 12T *1" [S42]="960G *1 + 1.92T *1 + 3.84T *1"
  [S43]="960G *1 + 7.68T *1" [S44]="2.4T *1 + 12T *1"
  [S45]="480G *2 + 7.68T *16" [S46]="1.92T *3 + 3.84T *3 + 7.68T *3"
  [S47]="480G *2 + 7.68T *2 + 18T *36" [S48]="960G *2 + 3.84T *1"
)

# Marketed disk-size buckets: decimal GB -> canonical label. detect.sh snaps a
# raw byte size to the nearest bucket (tolerance handled in idgen.sh).
# (A "960GB" SSD reports ~894GiB; a "3.84TB" SSD ~3576GiB, etc.)
declare -gA DISK_LABEL=(
  [480]="480G" [600]="600G" [800]="800G" [900]="900G" [960]="960G"
  [1200]="1.2T" [1600]="1.6T" [1700]="1.7T" [1920]="1.92T"
  [2400]="2.4T" [3200]="3.2T" [3840]="3.84T" [4000]="4T"
  [7680]="7.68T" [8000]="8T" [10000]="10T" [12000]="12T"
  [18000]="18T" [20000]="20T"
)
# ascending list of bucket sizes (GB) for nearest-match
DISK_BUCKETS=(480 800 900 960 1200 1600 1700 1920 2400 3200 3840 4000 7680 8000 10000 12000 18000 20000)

# Media override for describe-from-ID (when there is no live ROTA reading).
# Only the tiers the source spec explicitly tags as HDD need listing; the rest
# default to SSD. Key "<Sid>|<label>". detect mode overrides this with ROTA.
declare -gA DISK_MEDIA_SPEC=(
  ["S28|2.4T"]=HDD ["S29|2.4T"]=HDD ["S2A|2.4T"]=HDD
)
