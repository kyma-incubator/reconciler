from collections import OrderedDict
from functools import reduce
import os
import re
import semver  # pip install semver==2.10

print("Initializing script...")

# Paths
# os.chdir("../")
ROOT_DIR = os.getcwd()
GO_MOD = "{0}/go.mod".format(ROOT_DIR)
GO_SUM = "{0}/go.sum".format(ROOT_DIR)

# Regexes
RE_MODULE = "^module .*$"
RE_GO_COMPAT = "^go .*$"
RE_REPLACE_OPEN = "^replace \($"
RE_REQUIRE_OPEN = "^require \($"
RE_CLOSED = "\)"
RE_COMMENT = ".*\/\/.*"
RE_REPLACE = "^.+ => .+ v.+$"
RE_REQUIRE = "^(?!.+ => .+)(.+ v.+)$"

# Keywords
KW_MODULE = "module"
KW_GO = "go"
KW_REPLACE = "replace"
KW_REQUIRE = "require"
KW_ARROW = "=>"
KW_INDIRECT = "// indirect"
KW_OPEN = "("
KW_CLOSED = ")"
KW_COMMENT = "//"

# Log Messages
ERR_PATH = "Error: Can't find {0}"
ERR_PARSE = "Error parsing go.mod file: {0}"
ERR_PARSE_STM = ERR_PARSE.format("Statements population failed")
ERR_PARSE_LIN = ERR_PARSE.format("line {0}")

# Script checks
assert os.path.exists(ROOT_DIR), ERR_PATH.format(ROOT_DIR)
assert os.path.exists(GO_MOD), ERR_PATH.format(GO_MOD)
assert os.path.exists(GO_SUM), ERR_PATH.format(GO_SUM)

# Auxiliary data types


class ModuleData:
    """Metadata of go.mod module"""

    def __init__(self,
                 replace="",
                 version="",
                 comments=[],  # str
                 indirect=False):
        self.replace = replace
        self.version = version
        self.comments = comments
        self.indirect = indirect


class StatementData:
    """Metadata of go.mod statement (e.g. require, replace)"""

    def __init__(self,
                 name=""):
        self.name = name
        self.modules = OrderedDict()  # {name:str, data:ModuleData}


class GoModData:
    """Metadata of go.mod file"""

    def __init__(self,
                 module_name="",
                 go_compat=""):
        self.module_name = module_name
        self.go_compat = go_compat
        self.statements = []  # StatementData

# Auxiliary functions


def go_mod_parse():
    """
    Parses a go.mod file

    Returns:
    - result: GoModData
    """

    go_mod = open(GO_MOD, "r")
    lines = go_mod.readlines()
    go_mod.close()
    line_no = 0

    result = GoModData()
    inside_replace = False
    inside_require = False
    comments = []
    for line in lines:
        line = line.rstrip("\n")
        line_no += 1

        if re.match(RE_REQUIRE, line) and inside_require:
            assert len(result.statements) > 0, ERR_PARSE_STM

            line_split = line.split()
            assert len(line_split) >= 2, ERR_PARSE_LIN.format(line_no)

            name = line_split[0]
            version = line_split[1]
            indirect = (line_split[-2] + " " + line_split[-1]) == KW_INDIRECT
            result.statements[-1].modules[name] = ModuleData(
                "", version, comments, indirect
            )
            comments = []

        elif re.match(RE_REPLACE, line) and inside_replace:
            assert len(result.statements) > 0, ERR_PARSE_STM

            line_split = line.split()
            assert len(line_split) >= 4, ERR_PARSE_LIN.format(line_no)

            name = line_split[0]
            replace = line_split[2]
            version = line_split[3]
            result.statements[-1].modules[name] = ModuleData(
                replace, version, comments)
            comments = []

        elif re.match(RE_COMMENT, line):
            line = line.lstrip("\t/ ")
            comments.append(line)

        elif re.match(RE_REQUIRE_OPEN, line):
            inside_require = True
            result.statements.append(StatementData(KW_REQUIRE))

        elif re.match(RE_REPLACE_OPEN, line):
            inside_replace = True
            result.statements.append(StatementData(KW_REPLACE))

        elif re.match(RE_CLOSED, line):
            inside_require = False
            inside_replace = False

        elif re.match(RE_MODULE, line):
            result.module_name = line.split()[-1]

        elif re.match(RE_GO_COMPAT, line):
            result.go_compat = line.split()[-1]

    return result


def versions_cmp(v1, v2):
    """
    Compares 2 versions, based on the "Semantic Versioning 2.0.0" rules and requirements

    Parameters:
    - v1: str
    - v2: str

    Returns:
    - result: bool
        - v1 > v2 => result > 0
        - v1 < v2 => result < 0
        - v1 = v2 => result = 0
    """

    v1 = v1.lstrip("v")
    v2 = v2.lstrip("v")

    return semver.compare(v1, v2)


def drop_obsolete(go_mod_data):
    """
    Drops obsolete replace statements in a GoModData object and returns the number of dropped statements

    Parameters:
    - go_mod_data: GoModData

    Returns:
    - obsolete_no: int
    """

    statements_rq = [s for s in go_mod_data.statements
                     if s.name == KW_REQUIRE]
    statements_rp = [s for s in go_mod_data.statements
                     if s.name == KW_REPLACE]

    modules_rq = reduce(lambda d, src: d.update(src) or d,
                        [s_rq.modules for s_rq in statements_rq], {})
    modules_rp = reduce(lambda d, src: d.update(src) or d,
                        [s_rp.modules for s_rp in statements_rp], {})

    obsolete_no = 0
    m_to_delete = []
    for m_name, m_data in modules_rq.items():
        if (m_name in modules_rp) and\
                versions_cmp(modules_rp[m_name].version, m_data.version) < 0:
            m_to_delete.append(m_name)
            obsolete_no += 1

    for m_name in m_to_delete:
        for s_rp in statements_rp:
            if m_name in s_rp.modules:
                del s_rp.modules[m_name]

    return obsolete_no


# TODO: Improve performance (grep is pretty slow)
def drop_unreferenced(go_mod_data):
    """
    Drops unreferenced replace statements in a GoModData object and returns the number of dropped statements

    Parameters:
    - go_mod_data: GoModData

    Returns:
    - unreferenced_no: int
    """

    statements_rp = [s for s in go_mod_data.statements
                     if s.name == KW_REPLACE]

    unreferenced_no = 0
    for s_rp in statements_rp:
        for m_name in s_rp.modules:
            stream = os.popen("go mod graph | grep {0}".format(m_name))
            output = stream.read().strip()

            if len(output) <= 0:
                unreferenced_no += 1
                del s_rp.modules[m_name]

    return unreferenced_no


def go_mod_write(go_mod_data, file=GO_MOD):
    """
    Writes given GoModData in the go.mod file format to a given file

    Parameters:
    - go_mod_Data: GoModData
    - [optional] file: str (default: GO_MOD)
    """

    f = open(file, "w")

    f.write("{0} {1}\n\n".format(KW_MODULE, go_mod_data.module_name))
    f.write("{0} {1}\n\n".format(KW_GO, go_mod_data.go_compat))

    for statement in go_mod_data.statements:
        f.write("{0} {1}\n".format(statement.name, KW_OPEN))
        for m_name, m_data in statement.modules.items():
            # comments
            if len(m_data.comments) > 0:
                for comment in m_data.comments:
                    f.write("\t{0} {1}\n".format(KW_COMMENT, comment))

            # replace
            if statement.name == KW_REPLACE:
                f.write("\t{0} {1} {2} {3}\n".format(
                    m_name, KW_ARROW, m_data.replace, m_data.version
                ))
            # require
            elif statement.name == KW_REQUIRE:
                f.write("\t{0} {1}".format(m_name, m_data.version))

                # indirect
                if m_data.indirect:
                    f.write(" {0}".format(KW_INDIRECT))

                f.write("\n")
        f.write("{0}\n\n".format(KW_CLOSED))

    f.close()


print("Script successfully initialized")

# Main function
if __name__ == '__main__':
    # STEP 0: Build necessary data structures
    print("Parsing go.mod file...")
    go_mod_data = go_mod_parse()
    print("go.mod file successfully parsed")

    # STEP 1: Check for obsolete replace statements
    print("Checking for obsolete replace statements...")
    obsolete_no = drop_obsolete(go_mod_data)
    print("Successfully checked for obsolete replace statements: {0} statement/s dropped"
          .format("No" if obsolete_no is 0 else obsolete_no))

    # STEP 2: Check for unreferenced replace statements
    print("Checking for unreferenced replace statements...")
    unreferenced_no = drop_unreferenced(go_mod_data)
    print("Successfully checked for unreferenced replace statements: {0} statement/s dropped"
          .format("No" if unreferenced_no is 0 else unreferenced_no))

    # STEP 3: Rewrite go.mod file
    print("Rewriting go.mod file...")
    go_mod_write(go_mod_data)
    print("Successfully wrote data to go.mod file")
    print("Script executed successfully")
