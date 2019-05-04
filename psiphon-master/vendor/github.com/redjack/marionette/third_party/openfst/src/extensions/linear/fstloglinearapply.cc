
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
//
// Copyright 2005-2010 Google, Inc.
// Author: wuke

#include <fst/compat.h>
#include <fst/extensions/linear/linear-fst.h>
#include <fst/extensions/linear/loglinear-apply.h>
#include <fst/vector-fst.h>

DEFINE_bool(normalize, true, "Normalize to get posterior");

int main(int argc, char **argv) {
  string usage =
      "Applies an FST to another FST, treating the second as a log-linear "
      "model.\n\n  "
      "Usage: ";
  usage += argv[0];
  usage += " in.fst linear.fst [out.fst]\n";

  std::set_new_handler(FailedNewHandler);
  SET_FLAGS(usage.c_str(), &argc, &argv, true);
  if (argc < 3 || argc > 4) {
    ShowUsage();
    return 1;
  }

  string in_name = strcmp(argv[1], "-") != 0 ? argv[1] : "";
  string linear_name = (argc > 2 && (strcmp(argv[2], "-") != 0)) ? argv[2] : "";
  string out_name = argc > 3 ? argv[3] : "";

  if (in_name.empty() && linear_name.empty()) {
    LOG(ERROR) << argv[0] << ": Can't take both inputs from standard input.";
    return 1;
  }

  fst::StdFst *ifst1 = fst::StdFst::Read(in_name);
  if (!ifst1) return 1;

  fst::StdFst *ifst2 = fst::StdFst::Read(linear_name);
  if (!ifst2) return 1;

  fst::StdVectorFst ofst;

  LogLinearApply(*ifst1, *ifst2, &ofst, FLAGS_normalize);

  ofst.Write(out_name);

  return 0;
}
