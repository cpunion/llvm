//===- TargetBindings.cpp - Additional bindings for target ---------------===//
//
// Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
// See https://llvm.org/LICENSE.txt for license information.
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
//===----------------------------------------------------------------------===//

#include "TargetBindings.h"

#include "llvm/Config/llvm-config.h"
#include "llvm/Support/CBindingWrapping.h"
#include "llvm/Target/TargetMachine.h"

#if LLVM_VERSION_MAJOR < 18
#include "llvm/MC/TargetRegistry.h"
#include "llvm/Target/CodeGenCWrappers.h"
#include "llvm/Target/TargetOptions.h"

#if LLVM_VERSION_MAJOR < 16
#include "llvm/ADT/Optional.h"
#else
#include <optional>
#endif
#endif

using namespace llvm;

DEFINE_SIMPLE_CONVERSION_FUNCTIONS(TargetMachine, LLVMTargetMachineRef)

#if LLVM_VERSION_MAJOR < 18
DEFINE_SIMPLE_CONVERSION_FUNCTIONS(Target, LLVMTargetRef)

#if LLVM_VERSION_MAJOR < 16
template <typename T> using LLVMGoOptional = Optional<T>;
#else
template <typename T> using LLVMGoOptional = std::optional<T>;
#endif

static LLVMTargetMachineRef LLVMGoCreateTargetMachineWithLegacyOptions(
    LLVMTargetRef T, const char *Triple, const char *CPU, const char *Features,
    LLVMCodeGenOptLevel Level, LLVMRelocMode RelocMode, LLVMCodeModel CM,
    const char *ABIName, LLVMBool FunctionSections, LLVMBool DataSections,
    LLVMBool UniqueSectionNames) {
  LLVMGoOptional<Reloc::Model> RM;
  switch (RelocMode) {
  case LLVMRelocStatic:
    RM = Reloc::Static;
    break;
  case LLVMRelocPIC:
    RM = Reloc::PIC_;
    break;
  case LLVMRelocDynamicNoPic:
    RM = Reloc::DynamicNoPIC;
    break;
  case LLVMRelocROPI:
    RM = Reloc::ROPI;
    break;
  case LLVMRelocRWPI:
    RM = Reloc::RWPI;
    break;
  case LLVMRelocROPI_RWPI:
    RM = Reloc::ROPI_RWPI;
    break;
  default:
    break;
  }

  bool JIT = false;
  LLVMGoOptional<CodeModel::Model> CodeModel = unwrap(CM, JIT);

  CodeGenOpt::Level OptLevel;
  switch (Level) {
  case LLVMCodeGenLevelNone:
    OptLevel = CodeGenOpt::None;
    break;
  case LLVMCodeGenLevelLess:
    OptLevel = CodeGenOpt::Less;
    break;
  case LLVMCodeGenLevelAggressive:
    OptLevel = CodeGenOpt::Aggressive;
    break;
  default:
    OptLevel = CodeGenOpt::Default;
    break;
  }

  TargetOptions Options;
  Options.MCOptions.ABIName = ABIName;
  Options.FunctionSections = !!FunctionSections;
  Options.DataSections = !!DataSections;
  Options.UniqueSectionNames = !!UniqueSectionNames;
  return wrap(unwrap(T)->createTargetMachine(Triple, CPU, Features, Options, RM,
                                             CodeModel, OptLevel, JIT));
}
#endif

LLVMTargetMachineRef LLVMGoCreateTargetMachineWithOptions(
    LLVMTargetRef T, const char *Triple, const char *CPU, const char *Features,
    LLVMCodeGenOptLevel Level, LLVMRelocMode RelocMode, LLVMCodeModel CM,
    const char *ABIName, LLVMBool FunctionSections, LLVMBool DataSections,
    LLVMBool UniqueSectionNames) {
  if (!T)
    return nullptr;

  if (!ABIName)
    ABIName = "";

#if LLVM_VERSION_MAJOR >= 18
  LLVMTargetMachineOptionsRef Options = LLVMCreateTargetMachineOptions();
  LLVMTargetMachineOptionsSetCPU(Options, CPU);
  LLVMTargetMachineOptionsSetFeatures(Options, Features);
  LLVMTargetMachineOptionsSetABI(Options, ABIName);
  LLVMTargetMachineOptionsSetCodeGenOptLevel(Options, Level);
  LLVMTargetMachineOptionsSetRelocMode(Options, RelocMode);
  LLVMTargetMachineOptionsSetCodeModel(Options, CM);
  LLVMTargetMachineRef TM =
      LLVMCreateTargetMachineWithOptions(T, Triple, Options);
  LLVMDisposeTargetMachineOptions(Options);
#else
  LLVMTargetMachineRef TM = LLVMGoCreateTargetMachineWithLegacyOptions(
      T, Triple, CPU, Features, Level, RelocMode, CM, ABIName, FunctionSections,
      DataSections, UniqueSectionNames);
#endif
  if (!TM)
    return nullptr;

  TargetMachine *Machine = unwrap(TM);
  Machine->Options.FunctionSections = !!FunctionSections;
  Machine->Options.DataSections = !!DataSections;
  Machine->Options.UniqueSectionNames = !!UniqueSectionNames;
  return TM;
}

LLVMBool LLVMGoTargetMachineFunctionSections(LLVMTargetMachineRef TM) {
  TargetMachine *Machine = unwrap(TM);
  return Machine && Machine->Options.FunctionSections;
}

LLVMBool LLVMGoTargetMachineDataSections(LLVMTargetMachineRef TM) {
  TargetMachine *Machine = unwrap(TM);
  return Machine && Machine->Options.DataSections;
}

LLVMBool LLVMGoTargetMachineUniqueSectionNames(LLVMTargetMachineRef TM) {
  TargetMachine *Machine = unwrap(TM);
  return Machine && Machine->Options.UniqueSectionNames;
}
