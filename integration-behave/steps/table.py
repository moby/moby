def _column_indices(header):
    # A column is delimited by 2 or more consecutive spaces: this is how we
    # determine their respective width.
    result = [0]
    for i in range(len(header) - 2):
        triplet = header[i:i+3]
        if map(str.isspace, triplet) == [True, True, False]:
            result.append(i+2)
    result.append(len(header))
    return result


def _fixed_width_split(cols, line):
    return [line[cols[i]:cols[i+1]].strip() for i in range(len(cols) - 1)]


def parse(output):
    reader = (line for line in output.splitlines())

    # We need to account for the fact that headers and content values may have
    # whitespace, so splitting is not an option here: we detect the column
    # width, and use this for splitting.
    headers = next(reader)
    columns = _column_indices(headers)
    headers = _fixed_width_split(columns, headers)

    # Build a list of objects from the rest of the output.
    result = []
    for line in reader:
        result.append(dict(zip(headers, _fixed_width_split(columns, line))))
    return result

