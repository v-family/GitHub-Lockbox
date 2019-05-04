/*
 * Copyright (c) 2017, Psiphon Inc.
 * All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

// Logger exposes a logging interface that's compatible with
// psiphon/server.ContextLogger. This interface allows packages
// to implement logging that will integrate with psiphon/server
// without importing that package. Other implementations of
// Logger may also be provided.
type Logger interface {
	WithContext() LogContext
	WithContextFields(fields LogFields) LogContext
	LogMetric(metric string, fields LogFields)
}

// LogContext is interface-compatible with the return values from
// psiphon/server.ContextLogger.WithContext/WithContextFields.
type LogContext interface {
	Debug(args ...interface{})
	Info(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
}

// LogFields is type-compatible with psiphon/server.LogFields
// and logrus.LogFields.
type LogFields map[string]interface{}

// MetricsSource is an object that provides metrics to be logged
type MetricsSource interface {

	// GetMetrics returns a LogFields populated with
	// metrics from the MetricsSource
	GetMetrics() LogFields
}
