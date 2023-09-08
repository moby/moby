/*
 * ZLint Copyright 2021 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package util

import "math/big"

var bigIntPrimes = []*big.Int{
	big.NewInt(2), big.NewInt(3), big.NewInt(5), big.NewInt(7), big.NewInt(11), big.NewInt(13),
	big.NewInt(17), big.NewInt(19), big.NewInt(23), big.NewInt(29), big.NewInt(31), big.NewInt(37),
	big.NewInt(41), big.NewInt(43), big.NewInt(47), big.NewInt(53), big.NewInt(59), big.NewInt(61),
	big.NewInt(67), big.NewInt(71), big.NewInt(73), big.NewInt(79), big.NewInt(83), big.NewInt(89),
	big.NewInt(97), big.NewInt(101), big.NewInt(103), big.NewInt(107), big.NewInt(109), big.NewInt(113),
	big.NewInt(127), big.NewInt(131), big.NewInt(137), big.NewInt(139), big.NewInt(149), big.NewInt(151),
	big.NewInt(157), big.NewInt(163), big.NewInt(167), big.NewInt(173), big.NewInt(179), big.NewInt(181),
	big.NewInt(191), big.NewInt(193), big.NewInt(197), big.NewInt(199), big.NewInt(211), big.NewInt(223),
	big.NewInt(227), big.NewInt(229), big.NewInt(233), big.NewInt(239), big.NewInt(241), big.NewInt(251),
	big.NewInt(257), big.NewInt(263), big.NewInt(269), big.NewInt(271), big.NewInt(277), big.NewInt(281),
	big.NewInt(283), big.NewInt(293), big.NewInt(307), big.NewInt(311), big.NewInt(353), big.NewInt(359),
	big.NewInt(367), big.NewInt(373), big.NewInt(379), big.NewInt(383), big.NewInt(313), big.NewInt(317),
	big.NewInt(331), big.NewInt(337), big.NewInt(347), big.NewInt(349), big.NewInt(389), big.NewInt(397),
	big.NewInt(401), big.NewInt(409), big.NewInt(419), big.NewInt(421), big.NewInt(431), big.NewInt(433),
	big.NewInt(439), big.NewInt(443), big.NewInt(449), big.NewInt(457), big.NewInt(461), big.NewInt(463),
	big.NewInt(467), big.NewInt(479), big.NewInt(487), big.NewInt(491), big.NewInt(499), big.NewInt(503),
	big.NewInt(509), big.NewInt(521), big.NewInt(523), big.NewInt(541), big.NewInt(547), big.NewInt(557),
	big.NewInt(563), big.NewInt(569), big.NewInt(571), big.NewInt(577), big.NewInt(587), big.NewInt(593),
	big.NewInt(599), big.NewInt(601), big.NewInt(607), big.NewInt(613), big.NewInt(617), big.NewInt(619),
	big.NewInt(631), big.NewInt(641), big.NewInt(643), big.NewInt(647), big.NewInt(653), big.NewInt(659),
	big.NewInt(661), big.NewInt(673), big.NewInt(677), big.NewInt(683), big.NewInt(691), big.NewInt(701),
	big.NewInt(709), big.NewInt(719), big.NewInt(727), big.NewInt(733), big.NewInt(739), big.NewInt(743),
	big.NewInt(751),
}

var zero = big.NewInt(0)

func PrimeNoSmallerThan752(dividend *big.Int) bool {
	quotient := big.NewInt(0)
	mod := big.NewInt(0)
	for _, divisor := range bigIntPrimes {
		quotient.DivMod(dividend, divisor, mod)
		if mod.Cmp(zero) == 0 {
			return false
		}
	}
	return true
}
