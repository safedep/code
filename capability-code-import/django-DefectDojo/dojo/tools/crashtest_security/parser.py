__author__ = "phylu"

import json
import re

from defusedxml import ElementTree as ET

from dojo.models import Finding


class CrashtestSecurityJsonParser:

    """
    The objective of this class is to parse a json file generated by the crashtest security suite.

    @param file A proper json file generated by the crashtest security suite
    @param test The test to which the finding belongs
    """

    def get_findings(self, file, test):
        # Load the data
        tree = file.read()
        try:
            crashtest_scan = json.loads(str(tree, "utf-8"))
        except BaseException:
            crashtest_scan = json.loads(tree)

        # Extract the data from the data attribute if nested from original
        # request
        if "data" in crashtest_scan:
            crashtest_scan = crashtest_scan["data"]

        descriptions = self.create_descriptions_dict(
            crashtest_scan["descriptions"],
        )

        # Iterate scanner which contain the items
        items = []
        for scanner in crashtest_scan["findings"].values():
            # Iterate all findings of the scanner
            for finding in scanner:
                items.append(
                    self.generate_finding(finding, test, descriptions),
                )

                # Iterate all connected CVE findings if any
                if "cve_findings" in finding:
                    for cve_finding in finding["cve_findings"]:
                        items.append(
                            self.generate_cve_finding(cve_finding, test),
                        )
        return items

    def create_descriptions_dict(self, data):
        """
        Create a dict containing the finding descriptions

        @param data The list of finding descriptions

        @return descritpnios A dict of descriptions with their slug as key
        """
        # Create descriptions dictionary
        descriptions = {}
        for description in data:
            descriptions[description["slug"]] = description
        return descriptions

    def generate_finding(self, finding, test, descriptions):
        """
        Create a defect dojo Finding based on a crashtest security finding

        @param finding The crashtest security finding as dict from the json file
        @param test The test to which the finding belongs

        @return finding A finding as defect dojo Finding
        """
        description = descriptions[finding["finding_description_slug"]]
        severity = self.get_severity(description["baseScore"])
        impact = "CVSS Impact Score: {}".format(description["impact"])
        return Finding(
            title=description["title"],
            description=finding["information"],
            test=test,
            severity=severity,
            mitigation=description["how_to_fix"],
            references=description["reference_resolution"],
            active=True,
            verified=False,
            false_p=False,
            duplicate=False,
            out_of_scope=False,
            mitigated=None,
            impact=impact,
        )

    def generate_cve_finding(self, cve_finding, test):
        """
        Create a defect dojo Finding based on a crashtest security CVE finding

        @param finding The crashtest security CVE finding as dict from the json file
        @param test The test to which the finding belongs

        @return finding A finding as defect dojo Finding
        """
        severity = self.get_severity(cve_finding["cvss"])
        references = "https://nvd.nist.gov/vuln/detail/{}".format(
            cve_finding["cve_id"],
        )
        finding = Finding(
            title=cve_finding["cve_id"],
            description=cve_finding["information"],
            test=test,
            severity=severity,
            references=references,
            active=True,
            verified=False,
            false_p=False,
            duplicate=False,
            out_of_scope=False,
            mitigated=None,
        )
        finding.unsaved_vulnerability_ids = [cve_finding["cve_id"]]
        return finding

    def get_severity(self, cvss_base_score):
        """
        Convert a cvss base score to a defect dojo severity level

        @param cvss_base_score Score between 0 and 10

        @return severity A severity string (Info, Low, Medium, High or Critical)
        """
        if cvss_base_score == 0:
            return "Info"
        if cvss_base_score < 4:
            return "Low"
        if cvss_base_score < 7:
            return "Medium"
        if cvss_base_score < 9:
            return "High"
        return "Critical"


class CrashtestSecurityXmlParser:

    """
    The objective of this class is to parse an xml file generated by the crashtest security suite.

    @param xml_output A proper xml generated by the crashtest security suite
    """

    def get_findings(self, xml_output, test):
        tree = self.parse_xml(xml_output)

        if tree:
            return self.get_items(tree, test)
        return []

    def parse_xml(self, xml_output):
        """
        Open and parse an xml file.

        @return xml_tree An xml tree instance. None if error.
        """
        try:
            tree = ET.parse(xml_output)
        except SyntaxError as se:
            raise ValueError(se)

        return tree

    def get_items(self, tree, test):
        """@return items A list of Host instances"""
        items = []

        # Get all testcases
        for node in tree.findall(".//testcase"):
            # Only failed test cases contain a finding
            failure = node.find("failure")
            if failure is None:
                continue

            title = node.get("name")
            # Remove enumeration from title
            title = re.sub(r" \([0-9]*\)$", "", title)

            # Attache CVEs
            vulnerability_id = re.findall(r"CVE-\d{4}-\d{4,10}", title)[0] if "CVE" in title else None
            description = failure.get("message")
            severity = failure.get("type").capitalize()

            # This denotes an error of the scanner and not a vulnerability
            if severity == "Error":
                continue

            # This denotes a skipped scan and not a vulnerability
            if severity == "Skipped":
                continue

            find = Finding(
                title=title,
                description=description,
                test=test,
                severity=severity,
                mitigation="No mitigation provided",
                active=False,
                verified=False,
                false_p=False,
                duplicate=False,
                out_of_scope=False,
                mitigated=None,
                impact="No impact provided",
            )
            if vulnerability_id:
                find.unsaved_vulnerability_ids = [vulnerability_id]
            items.append(find)

        return items


class CrashtestSecurityParser:

    """SSLYze support JSON and XML"""

    def get_scan_types(self):
        return ["Crashtest Security JSON File", "Crashtest Security XML File"]

    def get_label_for_scan_types(self, scan_type):
        return scan_type  # no custom label for now

    def get_description_for_scan_types(self, scan_type):
        if scan_type == "Crashtest Security JSON File":
            return "JSON Report"
        return "XML Report"

    def get_findings(self, filename, test):
        if filename is None:
            return []

        if filename.name.lower().endswith(".xml"):
            return CrashtestSecurityXmlParser().get_findings(filename, test)
        if filename.name.lower().endswith(".json"):
            return CrashtestSecurityJsonParser().get_findings(filename, test)
        msg = "Unknown File Format"
        raise ValueError(msg)
