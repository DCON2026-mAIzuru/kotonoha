from pydantic import BaseModel, Field


class ColorRatios(BaseModel):
    green: float = 0.0
    yellow: float = 0.0
    brown_red: float = 0.0


class DiagnosisRatios(BaseModel):
    total: ColorRatios = Field(default_factory=ColorRatios)
    top: ColorRatios = Field(default_factory=ColorRatios)
    bottom: ColorRatios = Field(default_factory=ColorRatios)


class DiagnosisResult(BaseModel):
    request_id: str
    health_score: float
    status: str
    status_label: str
    severity: str
    severity_label: str
    detail: str
    ratios: DiagnosisRatios
    artifacts: dict[str, str]
    raw_stdout: str


class ErrorResponse(BaseModel):
    message: str

