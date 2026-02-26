// Copyright 2026 Unstable Build, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package debugrpc

import (
	"context"
	"io"

	"github.com/google/go-dap"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi/debugrpc"
	"github.com/unstablebuild/rune-go-sdk/joincontext"
	"google.golang.org/grpc"
)

var _ debugrpc.DebuggerServer = (*Server)(nil)
var _ io.Closer = (*Server)(nil)

// Server implements DebuggerServer by wrapping a debugapi.Debugger.
type Server struct {
	debugrpc.UnimplementedDebuggerServer
	debugger  debugapi.Debugger
	ctx       context.Context
	cancelCtx func()
}

// NewServer creates a new Server wrapping the given Debugger.
func NewServer(d debugapi.Debugger) *Server {
	ctx, cancelCtx := context.WithCancel(context.Background())
	return &Server{ctx: ctx, cancelCtx: cancelCtx, debugger: d}
}

// Register registers this server with the given gRPC server.
func (s *Server) Register(srv *grpc.Server) {
	debugrpc.RegisterDebuggerServer(srv, s)
}

// Initialize implements DebuggerServer.
func (s *Server) Initialize(ctx context.Context, req *debugrpc.InitializeRequest) (*debugrpc.InitializeResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &debugapi.InitializeRequestArguments{
		InitializeRequestArguments: dap.InitializeRequestArguments{
			ClientID:                     req.GetClientId(),
			ClientName:                   req.GetClientName(),
			AdapterID:                    req.GetAdapterId(),
			Locale:                       req.GetLocale(),
			LinesStartAt1:                req.GetLinesStartAt_1(),
			ColumnsStartAt1:              req.GetColumnsStartAt_1(),
			PathFormat:                   req.GetPathFormat(),
			SupportsVariableType:         req.GetSupportsVariableType(),
			SupportsVariablePaging:       req.GetSupportsVariablePaging(),
			SupportsRunInTerminalRequest: req.GetSupportsRunInTerminalRequest(),
			SupportsMemoryReferences:     req.GetSupportsMemoryReferences(),
			SupportsProgressReporting:    req.GetSupportsProgressReporting(),
			SupportsInvalidatedEvent:     req.GetSupportsInvalidatedEvent(),
			SupportsMemoryEvent:          req.GetSupportsMemoryEvent(),
		},
		InitializeOptions: req.GetInitializeOptions(),
	}

	caps, err := s.debugger.Initialize(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.InitializeResponse{
		Capabilities: capabilitiesToProto(caps),
	}, nil
}

// Launch implements DebuggerServer.
func (s *Server) Launch(ctx context.Context, req *debugrpc.LaunchRequest) (*debugrpc.LaunchResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := debugapi.LaunchRequestArguments{
		Program:     req.GetProgram(),
		Args:        req.GetArgs(),
		Cwd:         req.GetCwd(),
		Env:         req.GetEnv(),
		StopOnEntry: req.GetStopOnEntry(),
		NoDebug:     req.GetNoDebug(),
	}

	if err := s.debugger.Launch(ctx, args); err != nil {
		return nil, err
	}

	return &debugrpc.LaunchResponse{}, nil
}

// Attach implements DebuggerServer.
func (s *Server) Attach(ctx context.Context, req *debugrpc.AttachRequest) (*debugrpc.AttachResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := debugapi.AttachRequestArguments{
		PID:     int(req.GetPid()),
		Program: req.GetProgram(),
	}

	if err := s.debugger.Attach(ctx, args); err != nil {
		return nil, err
	}

	return &debugrpc.AttachResponse{}, nil
}

// ConfigurationDone implements DebuggerServer.
func (s *Server) ConfigurationDone(ctx context.Context, req *debugrpc.ConfigurationDoneRequest) (*debugrpc.ConfigurationDoneResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	if err := s.debugger.ConfigurationDone(ctx); err != nil {
		return nil, err
	}
	return &debugrpc.ConfigurationDoneResponse{}, nil
}

// Disconnect implements DebuggerServer.
func (s *Server) Disconnect(ctx context.Context, req *debugrpc.DisconnectRequest) (*debugrpc.DisconnectResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.DisconnectArguments{
		Restart:           req.GetRestart(),
		TerminateDebuggee: req.GetTerminateDebuggee(),
		SuspendDebuggee:   req.GetSuspendDebuggee(),
	}

	if err := s.debugger.Disconnect(ctx, args); err != nil {
		return nil, err
	}

	return &debugrpc.DisconnectResponse{}, nil
}

// Terminate implements DebuggerServer.
func (s *Server) Terminate(ctx context.Context, req *debugrpc.TerminateRequest) (*debugrpc.TerminateResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.TerminateArguments{
		Restart: req.GetRestart(),
	}

	if err := s.debugger.Terminate(ctx, args); err != nil {
		return nil, err
	}

	return &debugrpc.TerminateResponse{}, nil
}

// Restart implements DebuggerServer.
func (s *Server) Restart(ctx context.Context, req *debugrpc.RestartRequest) (*debugrpc.RestartResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	if err := s.debugger.Restart(ctx); err != nil {
		return nil, err
	}
	return &debugrpc.RestartResponse{}, nil
}

// SetBreakpoints implements DebuggerServer.
func (s *Server) SetBreakpoints(ctx context.Context, req *debugrpc.SetBreakpointsRequest) (*debugrpc.SetBreakpointsResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.SetBreakpointsArguments{
		Source:         sourceFromProto(req.GetSource()),
		Breakpoints:    sourceBreakpointsFromProto(req.GetBreakpoints()),
		SourceModified: req.GetSourceModified(),
	}

	bps, err := s.debugger.SetBreakpoints(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.SetBreakpointsResponse{
		Breakpoints: breakpointsToProto(bps),
	}, nil
}

// SetFunctionBreakpoints implements DebuggerServer.
func (s *Server) SetFunctionBreakpoints(ctx context.Context, req *debugrpc.SetFunctionBreakpointsRequest) (*debugrpc.SetFunctionBreakpointsResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.SetFunctionBreakpointsArguments{
		Breakpoints: functionBreakpointsFromProto(req.GetBreakpoints()),
	}

	bps, err := s.debugger.SetFunctionBreakpoints(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.SetFunctionBreakpointsResponse{
		Breakpoints: breakpointsToProto(bps),
	}, nil
}

// SetExceptionBreakpoints implements DebuggerServer.
func (s *Server) SetExceptionBreakpoints(ctx context.Context, req *debugrpc.SetExceptionBreakpointsRequest) (*debugrpc.SetExceptionBreakpointsResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.SetExceptionBreakpointsArguments{
		Filters: req.GetFilters(),
	}

	bps, err := s.debugger.SetExceptionBreakpoints(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.SetExceptionBreakpointsResponse{
		Breakpoints: breakpointsToProto(bps),
	}, nil
}

// Continue implements DebuggerServer.
func (s *Server) Continue(ctx context.Context, req *debugrpc.ContinueRequest) (*debugrpc.ContinueResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.ContinueArguments{
		ThreadId:     int(req.GetThreadId()),
		SingleThread: req.GetSingleThread(),
	}

	resp, err := s.debugger.Continue(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.ContinueResponse{
		AllThreadsContinued: resp.AllThreadsContinued,
	}, nil
}

// Pause implements DebuggerServer.
func (s *Server) Pause(ctx context.Context, req *debugrpc.PauseRequest) (*debugrpc.PauseResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.PauseArguments{
		ThreadId: int(req.GetThreadId()),
	}

	if err := s.debugger.Pause(ctx, args); err != nil {
		return nil, err
	}
	return &debugrpc.PauseResponse{}, nil
}

// Next implements DebuggerServer.
func (s *Server) Next(ctx context.Context, req *debugrpc.NextRequest) (*debugrpc.NextResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.NextArguments{
		ThreadId:     int(req.GetThreadId()),
		SingleThread: req.GetSingleThread(),
		Granularity:  dap.SteppingGranularity(req.GetGranularity()),
	}

	if err := s.debugger.Next(ctx, args); err != nil {
		return nil, err
	}
	return &debugrpc.NextResponse{}, nil
}

// StepIn implements DebuggerServer.
func (s *Server) StepIn(ctx context.Context, req *debugrpc.StepInRequest) (*debugrpc.StepInResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.StepInArguments{
		ThreadId:     int(req.GetThreadId()),
		SingleThread: req.GetSingleThread(),
		TargetId:     int(req.GetTargetId()),
		Granularity:  dap.SteppingGranularity(req.GetGranularity()),
	}

	if err := s.debugger.StepIn(ctx, args); err != nil {
		return nil, err
	}
	return &debugrpc.StepInResponse{}, nil
}

// StepOut implements DebuggerServer.
func (s *Server) StepOut(ctx context.Context, req *debugrpc.StepOutRequest) (*debugrpc.StepOutResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.StepOutArguments{
		ThreadId:     int(req.GetThreadId()),
		SingleThread: req.GetSingleThread(),
		Granularity:  dap.SteppingGranularity(req.GetGranularity()),
	}

	if err := s.debugger.StepOut(ctx, args); err != nil {
		return nil, err
	}
	return &debugrpc.StepOutResponse{}, nil
}

// StepBack implements DebuggerServer.
func (s *Server) StepBack(ctx context.Context, req *debugrpc.StepBackRequest) (*debugrpc.StepBackResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.StepBackArguments{
		ThreadId:     int(req.GetThreadId()),
		SingleThread: req.GetSingleThread(),
		Granularity:  dap.SteppingGranularity(req.GetGranularity()),
	}

	if err := s.debugger.StepBack(ctx, args); err != nil {
		return nil, err
	}
	return &debugrpc.StepBackResponse{}, nil
}

// ReverseContinue implements DebuggerServer.
func (s *Server) ReverseContinue(ctx context.Context, req *debugrpc.ReverseContinueRequest) (*debugrpc.ReverseContinueResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.ReverseContinueArguments{
		ThreadId:     int(req.GetThreadId()),
		SingleThread: req.GetSingleThread(),
	}

	if err := s.debugger.ReverseContinue(ctx, args); err != nil {
		return nil, err
	}
	return &debugrpc.ReverseContinueResponse{}, nil
}

// Threads implements DebuggerServer.
func (s *Server) Threads(ctx context.Context, req *debugrpc.ThreadsRequest) (*debugrpc.ThreadsResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	threads, err := s.debugger.Threads(ctx)
	if err != nil {
		return nil, err
	}

	return &debugrpc.ThreadsResponse{
		Threads: threadsToProto(threads),
	}, nil
}

// StackTrace implements DebuggerServer.
func (s *Server) StackTrace(ctx context.Context, req *debugrpc.StackTraceRequest) (*debugrpc.StackTraceResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.StackTraceArguments{
		ThreadId:   int(req.GetThreadId()),
		StartFrame: int(req.GetStartFrame()),
		Levels:     int(req.GetLevels()),
	}

	resp, err := s.debugger.StackTrace(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.StackTraceResponse{
		StackFrames: stackFramesToProto(resp.StackFrames),
		TotalFrames: int32(resp.TotalFrames),
	}, nil
}

// Scopes implements DebuggerServer.
func (s *Server) Scopes(ctx context.Context, req *debugrpc.ScopesRequest) (*debugrpc.ScopesResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.ScopesArguments{
		FrameId: int(req.GetFrameId()),
	}

	scopes, err := s.debugger.Scopes(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.ScopesResponse{
		Scopes: scopesToProto(scopes),
	}, nil
}

// Variables implements DebuggerServer.
func (s *Server) Variables(ctx context.Context, req *debugrpc.VariablesRequest) (*debugrpc.VariablesResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.VariablesArguments{
		VariablesReference: int(req.GetVariablesReference()),
		Filter:             req.GetFilter(),
		Start:              int(req.GetStart()),
		Count:              int(req.GetCount()),
	}

	vars, err := s.debugger.Variables(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.VariablesResponse{
		Variables: variablesToProto(vars),
	}, nil
}

// SetVariable implements DebuggerServer.
func (s *Server) SetVariable(ctx context.Context, req *debugrpc.SetVariableRequest) (*debugrpc.SetVariableResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.SetVariableArguments{
		VariablesReference: int(req.GetVariablesReference()),
		Name:               req.GetName(),
		Value:              req.GetValue(),
	}

	resp, err := s.debugger.SetVariable(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.SetVariableResponse{
		Value:              resp.Value,
		Type:               resp.Type,
		VariablesReference: int32(resp.VariablesReference),
		NamedVariables:     int32(resp.NamedVariables),
		IndexedVariables:   int32(resp.IndexedVariables),
	}, nil
}

// Source implements DebuggerServer.
func (s *Server) Source(ctx context.Context, req *debugrpc.SourceRequest) (*debugrpc.SourceResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.SourceArguments{
		SourceReference: int(req.GetSourceReference()),
		Source:          sourceFromProtoPtr(req.GetSource()),
	}

	resp, err := s.debugger.Source(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.SourceResponse{
		Content:  resp.Content,
		MimeType: resp.MimeType,
	}, nil
}

// Evaluate implements DebuggerServer.
func (s *Server) Evaluate(ctx context.Context, req *debugrpc.EvaluateRequest) (*debugrpc.EvaluateResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.EvaluateArguments{
		Expression: req.GetExpression(),
		FrameId:    int(req.GetFrameId()),
		Context:    req.GetContext(),
	}

	resp, err := s.debugger.Evaluate(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.EvaluateResponse{
		Result:             resp.Result,
		Type:               resp.Type,
		VariablesReference: int32(resp.VariablesReference),
		NamedVariables:     int32(resp.NamedVariables),
		IndexedVariables:   int32(resp.IndexedVariables),
		MemoryReference:    resp.MemoryReference,
	}, nil
}

// SetExpression implements DebuggerServer.
func (s *Server) SetExpression(ctx context.Context, req *debugrpc.SetExpressionRequest) (*debugrpc.SetExpressionResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.SetExpressionArguments{
		Expression: req.GetExpression(),
		Value:      req.GetValue(),
		FrameId:    int(req.GetFrameId()),
	}

	resp, err := s.debugger.SetExpression(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.SetExpressionResponse{
		Value:              resp.Value,
		Type:               resp.Type,
		VariablesReference: int32(resp.VariablesReference),
		NamedVariables:     int32(resp.NamedVariables),
		IndexedVariables:   int32(resp.IndexedVariables),
	}, nil
}

// Completions implements DebuggerServer.
func (s *Server) Completions(ctx context.Context, req *debugrpc.CompletionsRequest) (*debugrpc.CompletionsResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.CompletionsArguments{
		FrameId: int(req.GetFrameId()),
		Text:    req.GetText(),
		Column:  int(req.GetColumn()),
		Line:    int(req.GetLine()),
	}

	items, err := s.debugger.Completions(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.CompletionsResponse{
		Targets: completionItemsToProto(items),
	}, nil
}

// ExceptionInfo implements DebuggerServer.
func (s *Server) ExceptionInfo(ctx context.Context, req *debugrpc.ExceptionInfoRequest) (*debugrpc.ExceptionInfoResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.ExceptionInfoArguments{
		ThreadId: int(req.GetThreadId()),
	}

	resp, err := s.debugger.ExceptionInfo(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.ExceptionInfoResponse{
		ExceptionId: resp.ExceptionId,
		Description: resp.Description,
		BreakMode:   string(resp.BreakMode),
	}, nil
}

// Modules implements DebuggerServer.
func (s *Server) Modules(ctx context.Context, req *debugrpc.ModulesRequest) (*debugrpc.ModulesResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.ModulesArguments{
		StartModule: int(req.GetStartModule()),
		ModuleCount: int(req.GetModuleCount()),
	}

	resp, err := s.debugger.Modules(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.ModulesResponse{
		Modules:      modulesToProto(resp.Modules),
		TotalModules: int32(resp.TotalModules),
	}, nil
}

// LoadedSources implements DebuggerServer.
func (s *Server) LoadedSources(ctx context.Context, req *debugrpc.LoadedSourcesRequest) (*debugrpc.LoadedSourcesResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	sources, err := s.debugger.LoadedSources(ctx)
	if err != nil {
		return nil, err
	}

	return &debugrpc.LoadedSourcesResponse{
		Sources: sourcesToProto(sources),
	}, nil
}

// ReadMemory implements DebuggerServer.
func (s *Server) ReadMemory(ctx context.Context, req *debugrpc.ReadMemoryRequest) (*debugrpc.ReadMemoryResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.ReadMemoryArguments{
		MemoryReference: req.GetMemoryReference(),
		Offset:          int(req.GetOffset()),
		Count:           int(req.GetCount()),
	}

	resp, err := s.debugger.ReadMemory(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.ReadMemoryResponse{
		Address:         resp.Address,
		UnreadableBytes: int32(resp.UnreadableBytes),
		Data:            []byte(resp.Data),
	}, nil
}

// WriteMemory implements DebuggerServer.
func (s *Server) WriteMemory(ctx context.Context, req *debugrpc.WriteMemoryRequest) (*debugrpc.WriteMemoryResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.WriteMemoryArguments{
		MemoryReference: req.GetMemoryReference(),
		Offset:          int(req.GetOffset()),
		Data:            string(req.GetData()),
		AllowPartial:    req.GetAllowPartial(),
	}

	resp, err := s.debugger.WriteMemory(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.WriteMemoryResponse{
		Offset:       int64(resp.Offset),
		BytesWritten: int32(resp.BytesWritten),
	}, nil
}

// Disassemble implements DebuggerServer.
func (s *Server) Disassemble(ctx context.Context, req *debugrpc.DisassembleRequest) (*debugrpc.DisassembleResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.DisassembleArguments{
		MemoryReference:   req.GetMemoryReference(),
		Offset:            int(req.GetOffset()),
		InstructionOffset: int(req.GetInstructionOffset()),
		InstructionCount:  int(req.GetInstructionCount()),
		ResolveSymbols:    req.GetResolveSymbols(),
	}

	instructions, err := s.debugger.Disassemble(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.DisassembleResponse{
		Instructions: instructionsToProto(instructions),
	}, nil
}

// GotoTargets implements DebuggerServer.
func (s *Server) GotoTargets(ctx context.Context, req *debugrpc.GotoTargetsRequest) (*debugrpc.GotoTargetsResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.GotoTargetsArguments{
		Source: sourceFromProto(req.GetSource()),
		Line:   int(req.GetLine()),
		Column: int(req.GetColumn()),
	}

	targets, err := s.debugger.GotoTargets(ctx, args)
	if err != nil {
		return nil, err
	}

	return &debugrpc.GotoTargetsResponse{
		Targets: gotoTargetsToProto(targets),
	}, nil
}

// Goto implements DebuggerServer.
func (s *Server) Goto(ctx context.Context, req *debugrpc.GotoRequest) (*debugrpc.GotoResponse, error) {
	ctx, cancel := joincontext.New(ctx, s.ctx)
	defer cancel()
	args := &dap.GotoArguments{
		ThreadId: int(req.GetThreadId()),
		TargetId: int(req.GetTargetId()),
	}

	if err := s.debugger.Goto(ctx, args); err != nil {
		return nil, err
	}

	return &debugrpc.GotoResponse{}, nil
}

// Close closes all resources associated with this Server.
func (s *Server) Close() error {
	s.cancelCtx()
	return nil
}

func capabilitiesToProto(c *dap.Capabilities) *debugrpc.Capabilities {
	if c == nil {
		return nil
	}
	return &debugrpc.Capabilities{
		SupportsConfigurationDoneRequest:      c.SupportsConfigurationDoneRequest,
		SupportsFunctionBreakpoints:           c.SupportsFunctionBreakpoints,
		SupportsConditionalBreakpoints:        c.SupportsConditionalBreakpoints,
		SupportsHitConditionalBreakpoints:     c.SupportsHitConditionalBreakpoints,
		SupportsEvaluateForHovers:             c.SupportsEvaluateForHovers,
		SupportsStepBack:                      c.SupportsStepBack,
		SupportsSetVariable:                   c.SupportsSetVariable,
		SupportsRestartFrame:                  c.SupportsRestartFrame,
		SupportsGotoTargetsRequest:            c.SupportsGotoTargetsRequest,
		SupportsStepInTargetsRequest:          c.SupportsStepInTargetsRequest,
		SupportsCompletionsRequest:            c.SupportsCompletionsRequest,
		SupportsModulesRequest:                c.SupportsModulesRequest,
		SupportsRestartRequest:                c.SupportsRestartRequest,
		SupportsExceptionOptions:              c.SupportsExceptionOptions,
		SupportsValueFormattingOptions:        c.SupportsValueFormattingOptions,
		SupportsExceptionInfoRequest:          c.SupportsExceptionInfoRequest,
		SupportTerminateDebuggee:              c.SupportTerminateDebuggee,
		SupportSuspendDebuggee:                c.SupportSuspendDebuggee,
		SupportsDelayedStackTraceLoading:      c.SupportsDelayedStackTraceLoading,
		SupportsLoadedSourcesRequest:          c.SupportsLoadedSourcesRequest,
		SupportsLogPoints:                     c.SupportsLogPoints,
		SupportsTerminateThreadsRequest:       c.SupportsTerminateThreadsRequest,
		SupportsSetExpression:                 c.SupportsSetExpression,
		SupportsTerminateRequest:              c.SupportsTerminateRequest,
		SupportsDataBreakpoints:               c.SupportsDataBreakpoints,
		SupportsReadMemoryRequest:             c.SupportsReadMemoryRequest,
		SupportsWriteMemoryRequest:            c.SupportsWriteMemoryRequest,
		SupportsDisassembleRequest:            c.SupportsDisassembleRequest,
		SupportsCancelRequest:                 c.SupportsCancelRequest,
		SupportsBreakpointLocationsRequest:    c.SupportsBreakpointLocationsRequest,
		SupportsClipboardContext:              c.SupportsClipboardContext,
		SupportsSteppingGranularity:           c.SupportsSteppingGranularity,
		SupportsInstructionBreakpoints:        c.SupportsInstructionBreakpoints,
		SupportsExceptionFilterOptions:        c.SupportsExceptionFilterOptions,
		SupportsSingleThreadExecutionRequests: c.SupportsSingleThreadExecutionRequests,
	}
}

func sourceFromProto(s *debugrpc.Source) dap.Source {
	if s == nil {
		return dap.Source{}
	}
	return dap.Source{
		Name:             s.GetName(),
		Path:             s.GetPath(),
		SourceReference:  int(s.GetSourceReference()),
		PresentationHint: s.GetPresentationHint(),
		Origin:           s.GetOrigin(),
	}
}

func sourceFromProtoPtr(s *debugrpc.Source) *dap.Source {
	if s == nil {
		return nil
	}
	src := sourceFromProto(s)
	return &src
}

func sourceToProto(s *dap.Source) *debugrpc.Source {
	if s == nil {
		return nil
	}
	return &debugrpc.Source{
		Name:             s.Name,
		Path:             s.Path,
		SourceReference:  int32(s.SourceReference),
		PresentationHint: string(s.PresentationHint),
		Origin:           s.Origin,
	}
}

func sourcesToProto(sources []dap.Source) []*debugrpc.Source {
	result := make([]*debugrpc.Source, len(sources))
	for i := range sources {
		result[i] = sourceToProto(&sources[i])
	}
	return result
}

func sourceBreakpointsFromProto(bps []*debugrpc.SourceBreakpoint) []dap.SourceBreakpoint {
	result := make([]dap.SourceBreakpoint, len(bps))
	for i, bp := range bps {
		result[i] = dap.SourceBreakpoint{
			Line:         int(bp.GetLine()),
			Column:       int(bp.GetColumn()),
			Condition:    bp.GetCondition(),
			HitCondition: bp.GetHitCondition(),
			LogMessage:   bp.GetLogMessage(),
		}
	}
	return result
}

func functionBreakpointsFromProto(bps []*debugrpc.FunctionBreakpoint) []dap.FunctionBreakpoint {
	result := make([]dap.FunctionBreakpoint, len(bps))
	for i, bp := range bps {
		result[i] = dap.FunctionBreakpoint{
			Name:         bp.GetName(),
			Condition:    bp.GetCondition(),
			HitCondition: bp.GetHitCondition(),
		}
	}
	return result
}

func breakpointsToProto(bps []dap.Breakpoint) []*debugrpc.Breakpoint {
	result := make([]*debugrpc.Breakpoint, len(bps))
	for i, bp := range bps {
		result[i] = &debugrpc.Breakpoint{
			Id:                   int32(bp.Id),
			Verified:             bp.Verified,
			Message:              bp.Message,
			Source:               sourceToProto(bp.Source),
			Line:                 int32(bp.Line),
			Column:               int32(bp.Column),
			EndLine:              int32(bp.EndLine),
			EndColumn:            int32(bp.EndColumn),
			InstructionReference: bp.InstructionReference,
			Offset:               int32(bp.Offset),
		}
	}
	return result
}

func threadsToProto(threads []dap.Thread) []*debugrpc.Thread {
	result := make([]*debugrpc.Thread, len(threads))
	for i, t := range threads {
		result[i] = &debugrpc.Thread{
			Id:   int32(t.Id),
			Name: t.Name,
		}
	}
	return result
}

func stackFramesToProto(frames []dap.StackFrame) []*debugrpc.StackFrame {
	result := make([]*debugrpc.StackFrame, len(frames))
	for i, f := range frames {
		var moduleId int32
		if mid, ok := f.ModuleId.(int); ok {
			moduleId = int32(mid)
		}
		result[i] = &debugrpc.StackFrame{
			Id:                          int32(f.Id),
			Name:                        f.Name,
			Source:                      sourceToProto(f.Source),
			Line:                        int32(f.Line),
			Column:                      int32(f.Column),
			EndLine:                     int32(f.EndLine),
			EndColumn:                   int32(f.EndColumn),
			CanRestart:                  f.CanRestart,
			InstructionPointerReference: f.InstructionPointerReference,
			ModuleId:                    moduleId,
			PresentationHint:            f.PresentationHint,
		}
	}
	return result
}

func scopesToProto(scopes []dap.Scope) []*debugrpc.Scope {
	result := make([]*debugrpc.Scope, len(scopes))
	for i, s := range scopes {
		result[i] = &debugrpc.Scope{
			Name:               s.Name,
			PresentationHint:   s.PresentationHint,
			VariablesReference: int32(s.VariablesReference),
			NamedVariables:     int32(s.NamedVariables),
			IndexedVariables:   int32(s.IndexedVariables),
			Expensive:          s.Expensive,
			Source:             sourceToProto(s.Source),
			Line:               int32(s.Line),
			Column:             int32(s.Column),
			EndLine:            int32(s.EndLine),
			EndColumn:          int32(s.EndColumn),
		}
	}
	return result
}

func variablesToProto(vars []dap.Variable) []*debugrpc.Variable {
	result := make([]*debugrpc.Variable, len(vars))
	for i, v := range vars {
		var hint string
		if v.PresentationHint != nil {
			hint = v.PresentationHint.Kind
		}
		result[i] = &debugrpc.Variable{
			Name:               v.Name,
			Value:              v.Value,
			Type:               v.Type,
			PresentationHint:   hint,
			EvaluateName:       v.EvaluateName,
			VariablesReference: int32(v.VariablesReference),
			NamedVariables:     int32(v.NamedVariables),
			IndexedVariables:   int32(v.IndexedVariables),
			MemoryReference:    v.MemoryReference,
		}
	}
	return result
}

func instructionsToProto(instructions []dap.DisassembledInstruction) []*debugrpc.DisassembledInstruction {
	result := make([]*debugrpc.DisassembledInstruction, len(instructions))
	for i, inst := range instructions {
		result[i] = &debugrpc.DisassembledInstruction{
			Address:          inst.Address,
			InstructionBytes: inst.InstructionBytes,
			Instruction:      inst.Instruction,
			Symbol:           inst.Symbol,
			Location:         sourceToProto(inst.Location),
			Line:             int32(inst.Line),
			Column:           int32(inst.Column),
			EndLine:          int32(inst.EndLine),
			EndColumn:        int32(inst.EndColumn),
		}
	}
	return result
}

func completionItemsToProto(items []dap.CompletionItem) []*debugrpc.CompletionItem {
	result := make([]*debugrpc.CompletionItem, len(items))
	for i, item := range items {
		result[i] = &debugrpc.CompletionItem{
			Label:           item.Label,
			Text:            item.Text,
			SortText:        item.SortText,
			Detail:          item.Detail,
			Type:            string(item.Type),
			Start:           int32(item.Start),
			Length:          int32(item.Length),
			SelectionStart:  int32(item.SelectionStart),
			SelectionLength: int32(item.SelectionLength),
		}
	}
	return result
}

func modulesToProto(modules []dap.Module) []*debugrpc.Module {
	result := make([]*debugrpc.Module, len(modules))
	for i, m := range modules {
		var id string
		switch v := m.Id.(type) {
		case int:
			id = string(rune(v))
		case string:
			id = v
		}
		result[i] = &debugrpc.Module{
			Id:             id,
			Name:           m.Name,
			Path:           m.Path,
			IsOptimized:    m.IsOptimized,
			IsUserCode:     m.IsUserCode,
			Version:        m.Version,
			SymbolStatus:   m.SymbolStatus,
			SymbolFilePath: m.SymbolFilePath,
			DateTimeStamp:  m.DateTimeStamp,
			AddressRange:   m.AddressRange,
		}
	}
	return result
}

func gotoTargetsToProto(targets []dap.GotoTarget) []*debugrpc.GotoTarget {
	result := make([]*debugrpc.GotoTarget, len(targets))
	for i, t := range targets {
		result[i] = &debugrpc.GotoTarget{
			Id:                          int32(t.Id),
			Label:                       t.Label,
			Line:                        int32(t.Line),
			Column:                      int32(t.Column),
			EndLine:                     int32(t.EndLine),
			EndColumn:                   int32(t.EndColumn),
			InstructionPointerReference: t.InstructionPointerReference,
		}
	}
	return result
}
